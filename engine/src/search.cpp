#include "xiangqi/search_internal.hpp"
#include <algorithm>

namespace xiangqi::search_detail
{
int Search::root(int depth, std::vector<OrderedMove> &moves, Line &pv)
{
    int best = -legacy::maxScore - 2, alpha = -legacy::maxScore;
    bool moved = false;
    Line best_line;
    for (std::size_t i = 0; i < moves.size(); ++i)
    {
        check();
        auto best_index = i;
        for (auto j = i + 1; j < moves.size(); ++j)
            if (moves[j].score > moves[best_index].score)
                best_index = j;
        std::swap(moves[i], moves[best_index]);
        auto candidate = moves[i].hint;
        auto undo = board.make_move(candidate.move);
        push(candidate);
        Line line;
        int value;
        try
        {
            if (moved)
            {
                value = -run(-alpha - 1, -alpha, depth - 1, false, line);
                if (value > alpha)
                    value = -run(-legacy::maxScore, -alpha, depth - 1, true, line);
            }
            else
            {
                value = -run(-legacy::maxScore, -alpha, depth - 1, true, line);
                moved = true;
            }
        }
        catch (...)
        {
            pop();
            board.unmake_move(undo);
            throw;
        }
        pop();
        board.unmake_move(undo);
        check();
        moves[i].score = value;
        if (value > best)
        {
            best = value;
            alpha = std::max(alpha, value);
            best_line = {candidate};
            best_line.insert(best_line.end(), line.begin(), line.end());
        }
    }
    pv = std::move(best_line);
    return moved ? best : -legacy::maxScore;
}
} // namespace xiangqi::search_detail

namespace xiangqi
{
SearchResult RulesEngine::search(const SearchLimits &limits, const std::atomic_bool &cancelled,
                                 const std::function<void(const SearchInfo &)> &on_info)
{
    if (limits.max_depth == 0 || limits.max_depth > 32 || limits.time_ms == 0 || limits.time_ms > 3600000)
        throw EngineError("INVALID_ARGUMENT", "invalid search budget");
    auto root = board();
    prepare_evaluation(root, evaluation_phase_);
    std::vector<search_detail::Frame> past;
    auto rewind = root;
    for (auto i = history_.rbegin(); i != history_.rend(); ++i)
    {
        past.push_back({rewind.hash32, rewind.hash64, true, i->captured != nothing, false});
        rewind.unmake_move(*i);
    }
    past.push_back({rewind.hash32, rewind.hash64, false, false, false});
    std::reverse(past.begin(), past.end());
    search_detail::Search search(root, limits, cancelled, std::move(past));
    SearchResult result;
    if (root.pieces[16] == nothing || root.pieces[32] == nothing)
    {
        if (limits.rootMove)
            throw EngineError("ILLEGAL_MOVE", "root_move is not a legal root move");
        result.reason = StopReason::terminal;
        result.info.score = root.pieces[root.side == red ? 32 : 16] == nothing ? -legacy::maxScore : legacy::maxScore;
        return result;
    }
    auto candidates = ordered_moves(search.board, search.heuristics, limits.max_depth);
    std::vector<OrderedMove> moves;
    int initial_score = 100;
    for (auto candidate : candidates)
    {
        auto undo = search.board.make_move(candidate.hint.move);
        bool legal = !search.board.checked(undo.previous_side);
        search.board.unmake_move(undo);
        if (legal)
        {
            candidate.score = initial_score--;
            moves.push_back(candidate);
        }
    }
    // Search owns fresh TT and heuristics on every invocation. Restrict only the
    // root candidates, without applying the move as a new evaluation/history root.
    if (limits.rootMove)
    {
        const auto requested = *limits.rootMove;
        moves.erase(
            std::remove_if(moves.begin(), moves.end(), [&](const OrderedMove &move)
                           { return move.hint.move.from != requested.from || move.hint.move.to != requested.to; }),
            moves.end());
        if (moves.empty())
            throw EngineError("ILLEGAL_MOVE", "root_move is not a legal root move");
    }
    if (moves.empty())
    {
        result.reason = StopReason::terminal;
        result.info.score = -legacy::maxScore;
        return result;
    }
    result.has_move = true;
    result.best_move = moves.front().hint.move;
    result.info.pv = {result.best_move};
    auto first_depth = limits.max_depth >= 4 ? 4U : 1U;
    for (auto depth = first_depth; depth <= limits.max_depth; ++depth)
    {
        try
        {
            search.check();
            search_detail::Line line;
            int value = search.root(static_cast<int>(depth), moves, line);
            auto completed_ns = static_cast<std::uint64_t>(
                std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - search.started)
                    .count());
            search.check();
            std::vector<Move> pv;
            for (const auto &hint : line)
                pv.push_back(hint.move);
            if (pv.empty())
                throw EngineError("INTERNAL_ERROR", "completed root has no move");
            result.best_move = pv.front();
            result.info = {value, depth, search.nodes, search.elapsed(), pv};
            int index = static_cast<int>(depth) + 1;
            for (const auto &hint : line)
            {
                if (index < 0)
                    break;
                search.heuristics.remember(index--, hint);
            }
            result.info.evaluations = search.evaluations;
            result.info.cache_hits = search.cache_hits;
            result.info.cutoffs = search.cutoffs;
            result.info.completed_ns = completed_ns;
            if (on_info)
                on_info(result.info);
        }
        catch (const search_detail::Interrupted &stopped)
        {
            result.reason = stopped.reason;
            break;
        }
    }
    result.info.nodes = search.nodes;
    result.info.elapsed_ms = search.elapsed();
    result.info.evaluations = search.evaluations;
    result.info.cache_hits = search.cache_hits;
    result.info.cutoffs = search.cutoffs;
    return result;
}
} // namespace xiangqi
