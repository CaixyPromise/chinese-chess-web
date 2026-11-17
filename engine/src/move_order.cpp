#include "xiangqi/move_order.hpp"
#include <algorithm>
#include <cstring>

namespace xiangqi
{
BitBoard attack_sites(const Board &b, int piece)
{
    int src = b.pieces[piece];
    if (src == nothing)
        return {};
    const auto &t = Tables::get();
    auto row = b.rows[src / 9], col = b.cols[src % 9];
    switch ((piece_roles[piece] - 1) % 7 + 1)
    {
    case chariot:
        return t.rook_row[src][row] ^ t.rook_col[src][col] ^ t.quiet_row[src][row] ^ t.quiet_col[src][col];
    case cannon:
        return t.cannon_row[src][row] ^ t.cannon_col[src][col] ^ t.cannon_fake_row[src][row] ^
               t.cannon_fake_col[src][col];
    case knight:
        return t.knight_limited[src][(t.knight_legs[src] & b.occupied).knight_checksum()];
    case elephant:
        return t.elephant_limited[src][(t.elephant_legs[src] & b.occupied).elephant_checksum()];
    case king:
        return t.kings[src].value_or(BitBoard{});
    case guard:
        return t.guards[src].value_or(BitBoard{});
    case soldier:
        return t.soldiers[piece_side(piece)][src];
    }
    return {};
}
void SearchHeuristics::reward(const Board &b, Move move, int depth)
{
    auto role = legacy::chessRoles_eight[b.squares[move.from]];
    auto sum = static_cast<std::uint32_t>(history[role][move.to]) + (std::uint32_t{2} << (depth & 31));
    std::memcpy(&history[role][move.to], &sum, sizeof(sum)); // Defined Java int wrap.
}
void SearchHeuristics::remember(int depth, MoveHint hint)
{
    auto &pair = killers.at(static_cast<std::size_t>(depth));
    pair[1] = pair[0];
    pair[0] = hint;
}
void SearchHeuristics::clear()
{
    history = {};
    killers = {};
}
namespace
{
bool same(Move a, Move b)
{
    return a.from == b.from && a.to == b.to;
}
} // namespace
MovePicker::MovePicker(const Board &b, const SearchHeuristics &h, int depth,
                       const std::array<std::optional<MoveHint>, 2> &hints, bool quiet, bool checked)
    : board_(b), heuristics_(h), quiescent_(quiet), checked_(checked), stage_(quiet ? 4 : 0), depth_(depth)
{
    if (!quiet)
        preferred_ = {hints[0], hints[1], {}, {}};
}
std::optional<OrderedMove> MovePicker::select(std::vector<OrderedMove> &list)
{
    if (index_ >= list.size())
        return {};
    auto best = index_;
    for (auto i = index_ + 1; i < list.size(); ++i)
        if (list[i].score > list[best].score)
            best = i;
    std::swap(list[index_], list[best]);
    return list[index_++];
}
void MovePicker::generate(bool captures)
{
    const auto &b = board_;
    const auto &h = heuristics_;
    int begin = b.side == black ? 16 : 32;
    for (int offset = 1; offset <= 16; ++offset)
    {
        int piece = begin + offset % 16, src = b.pieces[piece];
        if (src == nothing)
            continue;
        const auto &tables = Tables::get();
        auto row = b.rows[src / 9], col = b.cols[src % 9];
        BitBoard destinations;
        switch ((piece_roles[piece] - 1) % 7 + 1)
        {
        case chariot:
            destinations = captures ? tables.rook_row[src][row] ^ tables.rook_col[src][col]
                                    : tables.quiet_row[src][row] ^ tables.quiet_col[src][col];
            break;
        case cannon:
            destinations = captures ? tables.cannon_row[src][row] ^ tables.cannon_col[src][col]
                                    : tables.quiet_row[src][row] ^ tables.quiet_col[src][col];
            break;
        case knight:
            destinations = tables.knight_limited[src][(tables.knight_legs[src] & b.occupied).knight_checksum()];
            break;
        case elephant:
            destinations = tables.elephant_limited[src][(tables.elephant_legs[src] & b.occupied).elephant_checksum()];
            break;
        case king:
            destinations = tables.kings[src].value_or(BitBoard{});
            break;
        case guard:
            destinations = tables.guards[src].value_or(BitBoard{});
            break;
        case soldier:
            destinations = tables.soldiers[b.side][src];
            break;
        }
        destinations = captures ? (destinations & b.personal[1 - b.side]) : destinations ^ (destinations & b.occupied);
        while (!destinations.empty())
        {
            int dest = destinations.first(b.side);
            destinations = destinations ^ BitBoard(dest);
            Move move{static_cast<std::uint8_t>(src), static_cast<std::uint8_t>(dest)};
            if (std::any_of(emitted_.begin(), emitted_.end(), [&](Move old) { return same(old, move); }))
                continue;
            bool protect = protected_.contains(dest);
            int score = h.history[legacy::chessRoles_eight[piece]][dest];
            int victim = b.squares[dest];
            if (victim != nothing)
            {
                int theirs =
                    b.evaluation.values[victim] + position_score(b.evaluation.phase, piece_roles[victim], dest);
                int ours = (!quiescent_ && !protect) ? -500
                                                     : b.evaluation.values[piece] +
                                                           position_score(b.evaluation.phase, piece_roles[piece], src);
                if ((!quiescent_ && theirs >= ours) || (quiescent_ && theirs >= 150))
                {
                    good_.push_back({move_hint(b, move), theirs - ours, protect, MoveStage::capture});
                    continue;
                }
            }
            if (!quiescent_ && !protect)
            {
                auto bits = static_cast<std::uint32_t>(score) + 256U;
                std::memcpy(&score, &bits, sizeof(bits));
            }
            general_.push_back({move_hint(b, move), score, protect, MoveStage::other});
        }
    }
}
std::optional<OrderedMove> MovePicker::next()
{
    while (stage_ < 4)
    {
        int stage = stage_++;
        auto hint = stage < 2 ? preferred_[stage] : heuristics_.killers.at(depth_)[stage - 2];
        auto equals = [&](const std::optional<MoveHint> &other)
        { return hint && other && same(hint->move, other->move); };
        bool duplicate = (stage >= 1 && equals(preferred_[0])) || (stage >= 2 && equals(preferred_[1])) ||
                         (stage == 3 && equals(heuristics_.killers.at(depth_)[0]));
        if (hint && valid_hint(board_, *hint) && !duplicate)
        {
            emitted_.push_back(hint->move);
            return OrderedMove{*hint, 0, false, static_cast<MoveStage>(stage)};
        }
    }
    if (stage_ == 4)
    {
        if (!generated_captures_)
        {
            if (!quiescent_)
                for (int p = (1 - board_.side) == black ? 16 : 32; p < ((1 - board_.side) == black ? 32 : 48); ++p)
                    protected_ = protected_ | attack_sites(board_, p);
            generate(true);
            generated_captures_ = true;
        }
        auto move = select(good_);
        if (move)
            return move;
        stage_ = 5;
        index_ = 0;
    }
    if (stage_ == 5)
    {
        if (quiescent_ && !checked_)
        {
            stage_ = 6;
            return {};
        }
        if (!generated_quiet_)
        {
            generate(false);
            generated_quiet_ = true;
        }
        auto move = select(general_);
        if (move)
            return move;
        stage_ = 6;
    }
    return {};
}
std::vector<OrderedMove> ordered_moves(const Board &b, const SearchHeuristics &h, int depth,
                                       const std::array<std::optional<MoveHint>, 2> &hints, bool quiet, bool checked)
{
    MovePicker picker(b, h, depth, hints, quiet, checked);
    std::vector<OrderedMove> result;
    while (auto move = picker.next())
        result.push_back(*move);
    return result;
}
} // namespace xiangqi
