#include "xiangqi/search_cache.hpp"
#include <algorithm>
#include <cstdlib>

namespace xiangqi
{
MoveHint move_hint(const Board &b, Move move)
{
    return {move, b.squares[move.from], b.squares[move.to]};
}
bool valid_hint(const Board &b, const MoveHint &h)
{
    return h.move.from < 90 && h.move.to < 90 && b.squares[h.move.from] == h.moving &&
           b.squares[h.move.to] == h.captured && b.pseudo_legal(h.move);
}
SearchCache::SearchCache(std::size_t slots) : slots_(slots)
{
    if (slots == 0 || (slots & (slots - 1)) != 0 || slots > (std::size_t{1} << 24))
        throw EngineError("INVALID_ARGUMENT", "cache slot count must be a bounded power of two");
    entries_.resize(slots * 2);
}
std::size_t SearchCache::slots_for_mib(std::uint32_t mib)
{
    if (mib < 1 || mib > 1024)
        throw EngineError("INVALID_ARGUMENT", "cache budget must be 1..1024 MiB");
    auto capacity = (static_cast<std::size_t>(mib) * 1024 * 1024) / (2 * sizeof(Bucket));
    std::size_t slots = 1;
    while (slots <= capacity / 2)
        slots *= 2;
    return slots;
}
std::size_t SearchCache::bytes() const
{
    return entries_.size() * sizeof(Bucket);
}
std::optional<int> SearchCache::value(const Entry &e, int depth, int alpha, int beta)
{
    int score = e.value;
    if (score > legacy::maxScore - 100)
        score -= depth - e.depth;
    else if (score < -legacy::maxScore + 100)
        score += depth - e.depth;
    else if (e.depth < depth)
        return {};
    if (e.bound == Bound::exact || (e.bound == Bound::beta && score >= beta) ||
        (e.bound == Bound::alpha && score <= alpha))
        return score;
    return {};
}
CacheProbe SearchCache::probe(std::uint32_t hash32, std::uint64_t hash64, int side, int depth, int alpha,
                              int beta) const
{
    const auto &bucket = entries_.at(static_cast<std::size_t>(side) * slots_ + (hash32 & (slots_ - 1)));
    CacheProbe result;
    const Entry *deep = nullptr;
    for (int lane = 0; lane < 2; ++lane)
    {
        const auto &e = bucket.lane[lane];
        if (!e.present || e.checksum != hash64)
            continue;
        auto score = value(e, depth, alpha, beta);
        if (score)
        {
            result.score = score;
            return result;
        }
        // Original getTranZobrist overwrites hint[0], not hint[1], on lane 1.
        result.hint = e.hint;
        if (lane == 0 || !deep || deep->depth < e.depth)
            result.ordering_value = e.value;
        if (lane == 0)
            deep = &e;
    }
    return result;
}
void SearchCache::store(std::uint32_t hash32, std::uint64_t hash64, int side, Bound bound, int value, int depth,
                        std::optional<MoveHint> hint)
{
    if ((value >= 8000 && value <= 9000) || (value >= -9000 && value <= -8000))
        return;
    auto &bucket = entries_.at(static_cast<std::size_t>(side) * slots_ + (hash32 & (slots_ - 1)));
    auto &deep = bucket.lane[0];
    std::optional<Entry> displaced;
    if (!deep.present || !deep.fresh || deep.depth <= depth)
    {
        if (deep.present)
            displaced = deep;
        deep = Entry{true, true, hash64, value, depth, bound, hint};
    }
    if (displaced)
        bucket.lane[1] = *displaced;
    else
    {
        auto &always = bucket.lane[1];
        if (hint)
            always.hint = hint; // Preserve original null-hint update semantics.
        always.present = always.fresh = true;
        always.checksum = hash64;
        always.value = value;
        always.depth = depth;
        always.bound = bound;
    }
}
void SearchCache::update_root_hint(std::uint32_t hash32, std::uint64_t hash64, int side, MoveHint hint)
{
    auto &entry = entries_.at(static_cast<std::size_t>(side) * slots_ + (hash32 & (slots_ - 1))).lane[0];
    // Original setRootTranZobrist mutates an existing lane only, even on a checksum collision.
    if (entry.present)
    {
        entry.checksum = hash64;
        entry.hint = hint;
    }
}
void SearchCache::age()
{
    // Age affects replacement, not lookup, as in cleanTranZobrist.
    for (auto &bucket : entries_)
        if (bucket.lane[0].present)
            bucket.lane[0].fresh = false;
}
void SearchCache::clear()
{
    std::fill(entries_.begin(), entries_.end(), Bucket{});
}
} // namespace xiangqi
