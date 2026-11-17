#include "xiangqi/search_internal.hpp"
#include <algorithm>
namespace xiangqi::search_detail
{
int Search::run(int alpha, int beta, int depth, bool pv_node, Line &pv)
{
    check();
    ++nodes;
    int distance = ply(), side = board.side;
    if (board.pieces[side == red ? 32 : 16] == nothing)
        return distance - legacy::maxScore;
    int best = distance - legacy::maxScore;
    if (best > beta)
        return best;
    if (distance >= 64)
        return fine();
    auto cached = cache.probe(board.hash32, board.hash64, side, depth, alpha, beta);
    if (cached.score)
    {
        ++cache_hits;
        return *cached.score;
    }
    bool checked = board.checked(side);
    if (repeated_check(checked))
        return legacy::LONGCHECKSCORE;
    auto previous = frames.back();
    if (!previous.null_move && previous.capture && drawn())
        return legacy::drawScore;
    if (checked)
        ++depth;
    if (depth <= 0)
        return quiet(alpha, beta, checked, pv);
    if (!previous.null_move && !checked && !pv_node && depth >= 2 && board.attack_defense[side][0] > 0)
    {
        reduction = depth <= 6 ? 2 : depth <= 8 ? 3 : 4;
        board.side = 1 - side;
        push_null();
        Line ignored;
        int value;
        try
        {
            value = -run(-beta, -beta + 1, depth - reduction - 1, false, ignored);
            if (value >= beta && !(board.attack_defense[side][0] > 2 && depth < 6))
                value = -run(-beta, -beta + 1, depth - reduction + 1, false, ignored);
        }
        catch (...)
        {
            pop();
            board.side = side;
            throw;
        }
        pop();
        board.side = side;
        if (value >= beta)
            return value;
    }
    if (depth >= 6 && pv_node && !cached.hint)
    {
        Line shallow;
        run(alpha, beta, depth - 2, pv_node, shallow);
        if (!shallow.empty())
        {
            cached.hint = shallow.front();
            cache.update_root_hint(board.hash32, board.hash64, side, *cached.hint);
        }
    }
    std::array<std::optional<MoveHint>, 2> hints{cached.hint, {}};
    MovePicker picker(board, heuristics, depth, hints, false, checked);
    bool moved = false;
    int searched = 0, current_alpha = alpha;
    Bound bound = Bound::alpha;
    int late_limit = pv_node ? 10 : 5;
    Line best_line;
    std::optional<MoveHint> best_hint;
    while (auto picked = picker.next())
    {
        const auto candidate = *picked;
        check();
        auto undo = board.make_move(candidate.hint.move);
        if (board.checked(side))
        {
            board.unmake_move(undo);
            continue;
        }
        if (!checked && !pv_node && depth < 6 && !danger(side) && searched >= late_limit &&
            static_cast<int>(depth * 1.29 * 155) - searched * depth * 10 + board.evaluation.base[side] -
                    board.evaluation.base[1 - side] <
                current_alpha)
        {
            board.unmake_move(undo);
            ++searched;
            continue;
        }
        push(candidate.hint);
        Line line;
        int value;
        try
        {
            if (!moved)
            {
                value = -run(-beta, -current_alpha, depth - 1, true, line);
                moved = true;
            }
            else
            {
                int reduction_depth = 2;
                if (!checked && depth >= 3 && searched >= late_limit)
                {
                    if (!danger(1 - side))
                    {
                        if (searched >= (late_limit + 5 + depth) * 2)
                            reduction_depth = 4;
                        else if (searched >= late_limit + 5 + depth)
                            reduction_depth = 3;
                    }
                    value = -run(-current_alpha - 1, -current_alpha, depth - reduction_depth, false, line);
                }
                else
                    value = current_alpha + 1;
                if (value > current_alpha)
                {
                    if (reduction_depth > 1)
                        value = -run(-current_alpha - 1, -current_alpha, depth - 1, false, line);
                    if (value > current_alpha)
                        value = -run(-beta, -current_alpha, depth - 1, true, line);
                }
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
        ++searched;
        if (value > best)
        {
            best = value;
            best_hint = candidate.hint;
            best_line = {candidate.hint};
            best_line.insert(best_line.end(), line.begin(), line.end());
            if (value >= beta)
            {
                if (!previous.null_move && candidate.stage != MoveStage::killer1 &&
                    candidate.stage != MoveStage::killer2)
                    heuristics.remember(depth, candidate.hint);
                bound = Bound::beta;
                ++cutoffs;
                break;
            }
            if (value > current_alpha)
            {
                current_alpha = value;
                bound = Bound::exact;
            }
        }
    }
    if (!moved)
        return distance - legacy::maxScore;
    pv = std::move(best_line);
    if (bound != Bound::alpha && best_hint)
        heuristics.reward(board, best_hint->move, depth);
    store(bound, best, depth, bound == Bound::alpha ? std::optional<MoveHint>{} : best_hint);
    return best;
}
} // namespace xiangqi::search_detail
