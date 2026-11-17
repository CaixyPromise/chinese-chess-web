#pragma once
#include "xiangqi/search_cache.hpp"

namespace xiangqi
{
enum class MoveStage
{
    table1 = 0,
    table2 = 1,
    killer1 = 2,
    killer2 = 3,
    capture = 4,
    other = 5
};
struct OrderedMove
{
    MoveHint hint;
    int score = 0;
    bool protected_target = false;
    MoveStage stage = MoveStage::other;
};
struct SearchHeuristics
{
    std::array<std::array<std::int32_t, 256>, 48> history{};
    std::array<std::array<std::optional<MoveHint>, 2>, 128> killers{};
    void reward(const Board &board, Move move, int depth);
    void remember(int depth, MoveHint hint);
    void clear();
};
class MovePicker
{
  public:
    MovePicker(const Board &board, const SearchHeuristics &heuristics, int depth,
               const std::array<std::optional<MoveHint>, 2> &hints = {}, bool quiescent = false, bool in_check = false);
    std::optional<OrderedMove> next();

  private:
    const Board &board_;
    const SearchHeuristics &heuristics_;
    std::array<std::optional<MoveHint>, 4> preferred_;
    bool quiescent_, checked_, generated_captures_ = false, generated_quiet_ = false;
    int stage_, depth_;
    std::size_t index_ = 0;
    BitBoard protected_;
    std::vector<OrderedMove> good_, general_;
    std::vector<Move> emitted_;
    void generate(bool captures);
    std::optional<OrderedMove> select(std::vector<OrderedMove> &list);
};
BitBoard attack_sites(const Board &board, int piece);
std::vector<OrderedMove> ordered_moves(const Board &board, const SearchHeuristics &heuristics, int depth,
                                       const std::array<std::optional<MoveHint>, 2> &hints = {}, bool quiescent = false,
                                       bool in_check = false);
} // namespace xiangqi
