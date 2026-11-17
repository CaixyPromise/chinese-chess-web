#include "xiangqi/engine.hpp"
#include "xiangqi/board.hpp"
#include "xiangqi/legacy_zobrist.hpp"

#include <iostream>
#include <type_traits>
#include <future>

namespace
{
const std::string initial = "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1";
void require(bool condition, const char *message)
{
    if (!condition)
        throw std::runtime_error(message);
}
void validate(const xiangqi::Board &board)
{
    require(board.consistent(), "incremental board differs from reconstructed state");
    std::uint32_t hash32 = 0;
    std::uint64_t hash64 = 0;
    for (int site = 0; site < 90; ++site)
        if (board.squares[site] != xiangqi::nothing)
        {
            int role = xiangqi::piece_roles[board.squares[site]];
            hash32 ^= xiangqi::zobrist32[site][role];
            hash64 ^= xiangqi::zobrist64[site][role];
        }
    require(hash32 == board.hash32 && hash64 == board.hash64, "incremental Zobrist mismatch");
}
std::uint64_t walk(xiangqi::Board &board, int depth)
{
    validate(board);
    if (depth == 0)
        return 1;
    auto before = board.state();
    auto moves = board.legal_moves();
    require(before == board.state(), "legal generation changed board");
    std::uint64_t count = 0;
    for (auto move : moves)
    {
        auto undo = board.make_move(move);
        count += walk(board, depth - 1);
        board.unmake_move(undo);
        require(before == board.state(), "undo changed board identity or counters");
    }
    return count;
}
void rules_tests()
{
    for (int site = 0; site < 108; ++site)
    {
        xiangqi::BitBoard bit(site);
        require(bit.count() == 1 && bit.contains(site), "bitboard segment error");
        require(bit.first(xiangqi::red) == site && bit.first(xiangqi::black) == site, "bit scan error");
        require((bit ^ bit).empty(), "bitboard xor error");
    }
    auto bits = xiangqi::BitBoard(1) | xiangqi::BitBoard(30) | xiangqi::BitBoard(32);
    require(bits.first(xiangqi::red) == 1 && bits.first(xiangqi::black) == 30, "legacy segment scan order changed");
    auto board = xiangqi::Board::from_fen(initial);
    require(board.fen() == initial, "FEN round trip failed");
    require(walk(board, 2) == 1920, "native restoration walk count differs");
    xiangqi::RulesEngine first, second;
    first.set_position({initial, {}, "legacy-java-v1"});
    second.set_position({initial, {}, "legacy-java-v1"});
    auto before = first.board().state();
    first.apply_move(xiangqi::parse_move("b2b9")); // Capture a knight; keep unique legacy piece IDs.
    validate(first.board());
    require(second.board().state() == before, "mutable state leaked between contexts");
    first.undo();
    require(first.board().state() == before, "capture undo failed");
    try
    {
        first.set_position({initial, {xiangqi::parse_move("b0c2"), xiangqi::parse_move("a9a0")}, "legacy-java-v1"});
        throw std::runtime_error("illegal history was accepted");
    }
    catch (const xiangqi::EngineError &error)
    {
        require(error.code() == "ILLEGAL_MOVE", "wrong history error");
    }
    require(first.board().state() == before, "failed replay replaced valid position");
    std::vector<xiangqi::Undo> trail;
    std::vector<std::string> snapshots;
    for (int ply = 0; ply < 24; ++ply)
    {
        auto moves = board.legal_moves();
        if (moves.empty())
            break;
        snapshots.push_back(board.state());
        trail.push_back(board.make_move(moves[(ply * 7) % moves.size()]));
        validate(board);
    }
    while (!trail.empty())
    {
        board.unmake_move(trail.back());
        trail.pop_back();
        validate(board);
        require(board.state() == snapshots.back(), "multi-step undo differs from saved state");
        snapshots.pop_back();
    }
    require(board.fen() == initial, "multi-step undo did not restore initial FEN");
    auto capture_king = xiangqi::Board::from_fen("4k4/9/9/9/9/4R4/9/9/9/4K4 r - - 0 1");
    auto king_before = capture_king.state();
    auto undo = capture_king.make_move(xiangqi::parse_move("e4e9"));
    validate(capture_king);
    require(capture_king.legal_moves().empty(), "captured king should have no moves");
    capture_king.unmake_move(undo);
    require(king_before == capture_king.state(), "king capture undo failed");
    std::vector<std::future<std::uint64_t>> workers;
    for (int i = 0; i < 4; ++i)
        workers.push_back(std::async(std::launch::async,
                                     []
                                     {
                                         auto local = xiangqi::Board::from_fen(initial);
                                         return walk(local, 2);
                                     }));
    for (auto &worker : workers)
        require(worker.get() == 1920, "parallel context mismatch");
    std::cout
        << "Native full-state restoration, capture/king undo, hash recomputation and four parallel contexts passed\n";
}
void evaluation_tests()
{
    auto board = xiangqi::Board::from_fen(initial);
    for (const auto &phase : {"auto", "middle", "end"})
    {
        xiangqi::prepare_evaluation(board, phase);
        auto before = board.state();
        auto root = xiangqi::evaluation_json(xiangqi::evaluate(board));
        for (auto move : board.legal_moves())
        {
            auto undo = board.make_move(move);
            validate(board);
            if (board.pieces[16] != xiangqi::nothing && board.pieces[32] != xiangqi::nothing)
            {
                auto report = xiangqi::evaluate(board);
                auto rebuilt = xiangqi::recompute_base(board, board.evaluation);
                for (int side = 0; side < 2; ++side)
                {
                    require(board.evaluation.base[side] == rebuilt[side] + board.evaluation.residual[side],
                            "evaluation residual invariant failed");
                    require(report.fields.at("scores")[side] == -report.fields.at("scores")[1 - side],
                            "evaluation perspectives differ");
                }
            }
            board.unmake_move(undo);
            require(board.state() == before, "evaluation undo failed");
            require(xiangqi::evaluation_json(xiangqi::evaluate(board)) == root, "evaluation has stale mutable state");
        }
    }
    auto sparse = xiangqi::Board::from_fen("4k4/9/9/9/4p4/9/9/9/9/4K4 r - - 0 1");
    require(sparse.evaluation.values[27] == 180, "dynamic pawn value differs");
    require(sparse.evaluation.values[19] == 664 && sparse.evaluation.values[21] == 436,
            "dynamic horse/cannon values differ");
    require(sparse.evaluation.residual[0] == -80, "cold Java initialization order changed");
    auto stable = sparse.state();
    xiangqi::RulesEngine other;
    other.set_position({initial, {}, "legacy-java-v1"});
    other.apply_move(xiangqi::parse_move("b2b9"));
    require(sparse.state() == stable && sparse.consistent(), "dynamic values leaked between games");
    std::vector<std::future<std::string>> workers;
    for (int i = 0; i < 4; ++i)
        workers.push_back(std::async(std::launch::async,
                                     [i]
                                     {
                                         auto local = xiangqi::Board::from_fen(initial);
                                         xiangqi::prepare_evaluation(local, i % 2 ? "middle" : "end");
                                         auto before = xiangqi::evaluation_json(xiangqi::evaluate(local));
                                         for (int n = 0; n < 20; ++n)
                                         {
                                             auto moves = local.legal_moves();
                                             auto undo = local.make_move(moves[(i + n) % moves.size()]);
                                             xiangqi::evaluate(local);
                                             local.unmake_move(undo);
                                             require(before == xiangqi::evaluation_json(xiangqi::evaluate(local)),
                                                     "parallel evaluator retained another game's features");
                                         }
                                         return before;
                                     }));
    auto end = workers[0].get(), middle = workers[1].get();
    require(workers[2].get() == end && workers[3].get() == middle && end != middle,
            "parallel evaluator results differ");
    std::cout << "Evaluation phase/dynamic-value/base-increment/undo and four parallel evaluator checks passed\n";
}
void search_tests()
{
    xiangqi::RulesEngine engine;
    engine.set_position({"rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1", {}});
    auto before = engine.board().fen();
    std::atomic_bool cancelled{false};
    xiangqi::SearchLimits limits{2, 2000, 30000, 16};
    auto result = engine.search(limits, cancelled, {});
    require(result.has_move && result.info.completed_depth > 0, "native search did not finish an iteration");
    xiangqi::RulesEngine replay;
    replay.set_position({before, {}});
    for (auto move : result.info.pv)
        replay.apply_move(move);
    require(engine.board().fen() == before, "search changed the accepted position");
    cancelled = true;
    auto stopped = engine.search(limits, cancelled, {});
    require(stopped.has_move && stopped.info.completed_depth == 0 && stopped.reason == xiangqi::StopReason::cancelled,
            "cancellation fallback is invalid");
    engine.apply_move(stopped.best_move);
    cancelled = false;
    limits.max_nodes = 1;
    auto bounded = engine.search(limits, cancelled, {});
    require(bounded.has_move && bounded.info.completed_depth == 0 && bounded.reason == xiangqi::StopReason::deadline,
            "node budget was ignored");
    engine.set_position({"4k4/9/9/9/9/4R4/9/9/9/4K4 r - - 0 1", {}});
    engine.apply_move(xiangqi::parse_move("e4e9"));
    auto terminal = engine.search(limits, cancelled, {});
    require(!terminal.has_move && terminal.reason == xiangqi::StopReason::terminal, "terminal root returned a move");
    std::cout << "Native PV legality, cancellation, node budget, state restoration and terminal checks passed\n";
}
} // namespace

int main()
{
    static_assert(std::has_virtual_destructor<xiangqi::Engine>::value, "polymorphic interface needs safe destruction");
    int checked = 0;
    for (int from = 0; from < 90; ++from)
        for (int to = 0; to < 90; ++to)
        {
            if (from == to)
                continue;
            xiangqi::Move move{static_cast<std::uint8_t>(from), static_cast<std::uint8_t>(to)};
            auto parsed = xiangqi::parse_move(xiangqi::format_move(move));
            if (parsed.from != move.from || parsed.to != move.to)
                return 1;
            ++checked;
        }
    xiangqi::validate_fen_structure("4k4/9/9/9/4p4/9/9/9/9/4K4 r - - 0 1");
    xiangqi::UnavailableEngine first, second;
    for (auto *engine : {&first, &second})
    {
        engine->reset();
        try
        {
            engine->legal_moves();
            return 1;
        }
        catch (const xiangqi::EngineError &error)
        {
            if (error.code() != "NOT_IMPLEMENTED")
                return 1;
        }
    }
    std::cout << checked << " coordinate round-trips and interface refusal checks passed\n";
    rules_tests();
    evaluation_tests();
    search_tests();
}
