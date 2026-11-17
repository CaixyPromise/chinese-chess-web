#include "xiangqi/move_order.hpp"
#include <future>
#include <iostream>
#include <stdexcept>
using namespace xiangqi;
void require(bool value, const char *message)
{
    if (!value)
        throw std::runtime_error(message);
}
int main()
{
    static_assert(sizeof(int) == 4, "legacy arithmetic requires 32-bit int");
    SearchCache cache(8), other(8);
    cache.store(7, 100, red, Bound::exact, 0, 4, {});
    auto hit = cache.probe(7, 100, red, 4, -100, 100);
    require(hit.score && *hit.score == 0, "last bucket or zero score was lost");
    require(!cache.probe(7, 101, red, 4, -100, 100).score, "checksum collision hit");
    require(!cache.probe(7, 100, black, 4, -100, 100).score, "side leaked into lookup");
    require(!other.probe(7, 100, red, 4, -100, 100).score, "cache leaked into another context");
    cache.age();
    require(cache.probe(7, 100, red, 4, -100, 100).score.has_value(), "aging incorrectly disabled lookup");
    cache.clear();
    require(!cache.probe(7, 100, red, 4, -100, 100).score, "clear retained an entry");
    for (auto mib : {1U, 16U, 128U})
    {
        SearchCache bounded(SearchCache::slots_for_mib(mib));
        require(bounded.bytes() <= std::size_t(mib) * 1024 * 1024, "cache exceeded its requested data budget");
    }
    {
        SearchCache aligned(SearchCache::slots_for_mib(128));
        require(aligned.slots() == 524288, "formal cache index count differs from Java");
        std::cout << "128MiB request: " << aligned.slots() << " indices, " << aligned.bytes()
                  << " bytes of table data\n";
    }
    auto board = Board::from_fen("rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1");
    prepare_evaluation(board);
    auto before = board.state();
    auto move = parse_move("b0c2");
    auto hint = move_hint(board, move);
    require(valid_hint(board, hint), "fresh hint rejected");
    SearchHeuristics h;
    h.reward(board, move, 30);
    require(h.history[knight][move.to] == INT32_MIN, "Java shift wrapping differs");
    h.reward(board, move, 30);
    require(h.history[knight][move.to] == 0, "Java addition wrapping differs");
    h.remember(4, hint);
    auto list = ordered_moves(board, h, 4, {hint, hint});
    require(list.size() == 44, "preferred duplicates removed valid moves");
    int duplicates = 0;
    for (auto entry : list)
        if (entry.hint.move.from == move.from && entry.hint.move.to == move.to)
            ++duplicates;
    require(duplicates == 1 && list.front().stage == MoveStage::table1, "preferred order or deduplication differs");
    require(before == board.state(), "ordering changed mutable board");
    auto undo = board.make_move(move);
    require(!valid_hint(board, hint), "stale piece identity accepted");
    board.unmake_move(undo);
    std::vector<std::future<void>> parallel;
    for (int i = 0; i < 4; ++i)
        parallel.push_back(std::async(
            std::launch::async,
            [i, board, move]
            {
                SearchCache local(8);
                SearchHeuristics local_h;
                for (int n = 0; n < 100; ++n)
                {
                    local.store(2, 99, red, Bound::exact, i + 1, 4, {});
                    local_h.reward(board, move, 1);
                    require(local.probe(2, 99, red, 4, -100, 100).score == i + 1, "parallel caches crossed contexts");
                }
                require(local_h.history[knight][move.to] == 400, "parallel history crossed contexts");
            }));
    for (auto &worker : parallel)
        worker.get();
    std::cout
        << "Search cache bounds/replacement identity, ordering, Java arithmetic and parallel isolation checks passed\n";
}
