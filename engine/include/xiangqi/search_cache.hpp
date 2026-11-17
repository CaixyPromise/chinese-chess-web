#pragma once
#include "xiangqi/board.hpp"
#include <optional>

namespace xiangqi
{
struct MoveHint
{
    Move move{};
    int moving = nothing, captured = nothing;
};
MoveHint move_hint(const Board &board, Move move);
bool valid_hint(const Board &board, const MoveHint &hint);

enum class Bound
{
    beta = 1,
    alpha = 2,
    exact = 3
};
struct CacheProbe
{
    std::optional<int> score;
    std::optional<MoveHint> hint;
    int ordering_value = 0;
};
// Both replacement lanes and both sides are private to one search context.
class SearchCache
{
  public:
    explicit SearchCache(std::size_t slots);
    static std::size_t slots_for_mib(std::uint32_t mib);
    CacheProbe probe(std::uint32_t hash32, std::uint64_t hash64, int side, int depth, int alpha, int beta) const;
    void store(std::uint32_t hash32, std::uint64_t hash64, int side, Bound bound, int value, int depth,
               std::optional<MoveHint> hint);
    void update_root_hint(std::uint32_t hash32, std::uint64_t hash64, int side, MoveHint hint);
    void age();
    void clear();
    std::size_t slots() const
    {
        return slots_;
    }
    std::size_t bytes() const;

  private:
    struct Entry
    {
        bool present = false, fresh = true;
        std::uint64_t checksum = 0;
        int value = 0, depth = 0;
        Bound bound = Bound::alpha;
        std::optional<MoveHint> hint;
    };
    struct Bucket
    {
        std::array<Entry, 2> lane;
    };
    std::size_t slots_;
    std::vector<Bucket> entries_;
    static std::optional<int> value(const Entry &entry, int depth, int alpha, int beta);
};
} // namespace xiangqi
