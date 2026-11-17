#pragma once
#include "xiangqi/move_order.hpp"
#include <chrono>

namespace xiangqi::search_detail
{
using Line = std::vector<MoveHint>;
struct Frame
{
    std::uint32_t hash32;
    std::uint64_t hash64;
    bool has_move, capture, null_move;
};
struct Interrupted
{
    StopReason reason;
};
class Search
{
  public:
    Board board;
    const SearchLimits &limits;
    const std::atomic_bool &cancelled;
    SearchCache cache;
    SearchHeuristics heuristics;
    std::vector<Frame> frames;
    std::size_t root_index;
    std::chrono::steady_clock::time_point started;
    std::uint64_t nodes = 0, evaluations = 0, cache_hits = 0, cutoffs = 0;
    int reduction = 0;
    Search(Board root, const SearchLimits &budget, const std::atomic_bool &stop, std::vector<Frame> past);
    std::uint64_t elapsed() const;
    int ply() const
    {
        return static_cast<int>(frames.size() - root_index - 1);
    }
    void check() const;
    int run(int alpha, int beta, int depth, bool pv_node, Line &pv);
    int quiet(int alpha, int beta, bool in_check, Line &pv);
    int root(int depth, std::vector<OrderedMove> &moves, Line &pv);
    int fine();
    bool repeated_check(bool checked) const;
    bool drawn() const;
    bool danger(int side) const;
    void push(MoveHint move);
    void push_null();
    void pop()
    {
        frames.pop_back();
    }
    void store(Bound bound, int value, int depth, std::optional<MoveHint> hint);
};
} // namespace xiangqi::search_detail
