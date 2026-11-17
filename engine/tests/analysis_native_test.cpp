#include "xiangqi/board.hpp"
#include <atomic>
#include <cassert>
#include <iostream>

int main()
{
    xiangqi::RulesEngine engine;
    engine.set_position({"rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1", {}});
    const auto before = engine.board().state();
    std::atomic_bool cancelled{false};
    xiangqi::SearchLimits limits{3, 30000, 0, 128};
    auto original = engine.search(limits, cancelled, {});
    for (auto move : {xiangqi::Move{255, 255}, xiangqi::parse_move("a0a9")})
    {
        limits.rootMove = move;
        bool rejected = false;
        try
        {
            engine.search(limits, cancelled, {});
        }
        catch (const xiangqi::EngineError &error)
        {
            rejected = error.code() == "ILLEGAL_MOVE";
        }
        assert(rejected && engine.board().state() == before);
    }
    limits.rootMove = xiangqi::parse_move("b0c2");
    auto forced = engine.search(limits, cancelled, {});
    assert(xiangqi::format_move(forced.best_move) == "b0c2" && forced.info.completed_depth == 3);
    cancelled = true;
    auto stopped = engine.search(limits, cancelled, {});
    assert(stopped.reason == xiangqi::StopReason::cancelled && stopped.info.completed_depth == 0);
    assert(xiangqi::format_move(stopped.best_move) == "b0c2");
    cancelled = false;
    limits.rootMove.reset();
    auto subsequent = engine.search(limits, cancelled, {});
    assert(subsequent.info.nodes == original.info.nodes && subsequent.info.score == original.info.score);
    assert(xiangqi::format_move(subsequent.best_move) == xiangqi::format_move(original.best_move));
    assert(engine.board().state() == before);
    std::cout << "Native root restriction, illegal-input restoration, cancellation and cold-root parity passed\n";
}
