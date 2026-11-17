#include "xiangqi/search_internal.hpp"
namespace xiangqi::search_detail
{
Search::Search(Board root, const SearchLimits &budget, const std::atomic_bool &stop, std::vector<Frame> past)
    : board(std::move(root)), limits(budget), cancelled(stop), cache(SearchCache::slots_for_mib(budget.hash_mib)),
      frames(std::move(past)), root_index(frames.size() - 1), started(std::chrono::steady_clock::now())
{
}
std::uint64_t Search::elapsed() const
{
    return static_cast<std::uint64_t>(
        std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - started).count());
}
void Search::check() const
{
    if (cancelled.load(std::memory_order_relaxed))
        throw Interrupted{StopReason::cancelled};
    if (elapsed() >= limits.time_ms || (limits.max_nodes && nodes >= limits.max_nodes))
        throw Interrupted{StopReason::deadline};
}
int Search::fine()
{
    ++evaluations;
    return evaluate_score(board);
}
bool Search::drawn() const
{
    return board.attack_defense[red][0] == 0 && board.attack_defense[black][0] == 0;
}
bool Search::repeated_check(bool checked) const
{
    if (!checked || frames.size() < 2)
        return false;
    const auto &current = frames.back();
    for (std::size_t i = frames.size() - 1; i > 0;)
    {
        const auto &old = frames[--i];
        if (!old.has_move)
            break;
        if (old.hash32 == current.hash32 && old.hash64 == current.hash64)
            return true;
        if (old.capture)
            break;
    }
    return false;
}
bool Search::danger(int side) const
{
    int opponent = 1 - side;
    auto pieces = board.roles[role_for(opponent, chariot)] ^ board.roles[role_for(opponent, knight)] ^
                  board.roles[role_for(opponent, cannon)];
    int count = 0;
    for (int site = 0; site < 90; ++site)
        if (pieces.contains(site))
        {
            int row = site / 9, col = site % 9;
            if (side == black ? (row < 3 || (row == 3 && col >= 3 && col <= 5))
                              : (row >= 7 || (row == 6 && col >= 3 && col <= 5)))
                ++count;
        }
    return count >= 3;
}
void Search::push(MoveHint hint)
{
    frames.push_back({board.hash32, board.hash64, true, hint.captured != nothing, false});
}
void Search::push_null()
{
    frames.push_back({board.hash32, board.hash64, false, false, true});
}
void Search::store(Bound bound, int value, int depth, std::optional<MoveHint> hint)
{
    cache.store(board.hash32, board.hash64, board.side, bound, value, depth, hint);
}
} // namespace xiangqi::search_detail
