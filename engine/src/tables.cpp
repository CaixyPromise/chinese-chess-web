#include "xiangqi/board.hpp"
#include <algorithm>

namespace xiangqi
{
BitBoard BitBoard::operator&(const BitBoard &b) const
{
    BitBoard out;
    for (int i = 0; i < 4; ++i)
        out.words[i] = words[i] & b.words[i];
    return out;
}
BitBoard BitBoard::operator|(const BitBoard &b) const
{
    BitBoard out;
    for (int i = 0; i < 4; ++i)
        out.words[i] = words[i] | b.words[i];
    return out;
}
BitBoard BitBoard::operator^(const BitBoard &b) const
{
    BitBoard out;
    for (int i = 0; i < 4; ++i)
        out.words[i] = words[i] ^ b.words[i];
    return out;
}
bool BitBoard::empty() const
{
    return (words[0] | words[1] | words[2] | words[3]) == 0;
}
bool BitBoard::contains(int site) const
{
    return site >= 0 && site < 108 && (words[site / 27] & (std::uint32_t{1} << (site % 27))) != 0;
}
int BitBoard::count() const
{
    int total = 0;
    for (auto word : words)
        total += __builtin_popcount(word);
    return total;
}
int BitBoard::first(int side) const
{
    for (int i = 0; i < 4; ++i)
    {
        int segment = side == red ? i : 3 - i;
        if (words[segment])
            return segment * 27 + __builtin_ctz(words[segment]);
    }
    return -1;
}
int BitBoard::knight_checksum() const
{
    auto t = words[0] ^ words[1] ^ words[2] ^ words[3];
    return (t & 0x7f) + ((t >> 7) & 0x7f) + ((t >> 14) & 0x7f) + ((t >> 21) & 0x7f);
}
int BitBoard::elephant_checksum() const
{
    auto t = words[0] ^ words[1] ^ words[2] ^ words[3];
    return (t & 0x7f) + ((t >> 6) & 0x7f) + ((t >> 13) & 0x7f) + ((t >> 19) & 0x7f);
}

namespace
{
using Jump = std::pair<int, int>; // destination, blocking square
bool inside(int row, int col)
{
    return row >= 0 && row < 10 && col >= 0 && col < 9;
}
Tables::Grid grid(int width)
{
    return Tables::Grid(90, std::vector<BitBoard>(width));
}

void jumps(int site, const std::vector<Jump> &moves, BitBoard &legs, std::vector<BitBoard> &limited, bool horse,
           std::array<int, 200> *mobility)
{
    std::vector<int> unique;
    for (const auto &move : moves)
    {
        legs = legs | BitBoard(move.second);
        if (std::find(unique.begin(), unique.end(), move.second) == unique.end())
            unique.push_back(move.second);
    }
    auto store = [&](std::size_t free_mask)
    {
        BitBoard blocked, attacks;
        for (std::size_t i = 0; i < unique.size(); ++i)
            if (!(free_mask & (std::size_t{1} << i)))
                blocked = blocked | BitBoard(unique[i]);
        for (const auto &move : moves)
            if (!blocked.contains(move.second))
                attacks = attacks | BitBoard(move.first);
        int key = horse ? blocked.knight_checksum() : blocked.elephant_checksum();
        if (key >= 200)
            throw std::logic_error("legacy jump key out of range at " + std::to_string(site));
        limited[key] = attacks;
        if (mobility)
            (*mobility)[key] = attacks.count();
    };
    // Preserve Java's increasing-size, lexicographic free-leg combinations.
    // Its elephant checksum has collisions at some off-domain squares, making
    // the last assignment significant. The all-blocked entry stays zero.
    std::function<void(std::size_t, int, std::size_t)> combinations;
    combinations = [&](std::size_t start, int remaining, std::size_t mask)
    {
        if (remaining == 0)
        {
            store(mask);
            return;
        }
        for (std::size_t i = start; i < unique.size(); ++i)
            combinations(i + 1, remaining - 1, mask | (std::size_t{1} << i));
    };
    for (int count = 1; count <= static_cast<int>(unique.size()); ++count)
    {
        combinations(0, count, 0);
    }
}

void ray_tables(int site, bool vertical, Tables::Grid &rook, Tables::Grid &quiet, Tables::Grid &cannon,
                Tables::Grid &fake, Tables::Grid &more)
{
    int length = vertical ? 10 : 9;
    int source = vertical ? site / 9 : site % 9;
    for (int mask = 0; mask < (1 << length); ++mask)
    {
        if (!(mask & (1 << (length - 1 - source))))
            continue;
        for (int step : {-1, 1})
        {
            int blockers = 0;
            for (int n = source + step; n >= 0 && n < length; n += step)
            {
                int dest = vertical ? n * 9 + site % 9 : site / 9 * 9 + n;
                BitBoard bit(dest);
                if (mask & (1 << (length - 1 - n)))
                {
                    ++blockers;
                    if (blockers == 1)
                        rook[site][mask] = rook[site][mask] | bit;
                    if (blockers == 2)
                        cannon[site][mask] = cannon[site][mask] | bit;
                    if (blockers == 3)
                    {
                        more[site][mask] = more[site][mask] | bit;
                        break;
                    }
                }
                else
                {
                    if (blockers == 0)
                        quiet[site][mask] = quiet[site][mask] | bit;
                    if (blockers == 1)
                        fake[site][mask] = fake[site][mask] | bit;
                }
            }
        }
    }
}
} // namespace

Tables::Tables()
    : knight_limited(grid(200)), elephant_limited(grid(200)), rook_row(grid(512)), rook_col(grid(1024)),
      quiet_row(grid(512)), quiet_col(grid(1024)), cannon_row(grid(512)), cannon_col(grid(1024)),
      cannon_fake_row(grid(512)), cannon_fake_col(grid(1024)), cannon_more_row(grid(512)), cannon_more_col(grid(1024))
{
    constexpr int horse_delta[8][2] = {{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2}, {1, -2}, {1, 2}, {2, -1}, {2, 1}};
    constexpr int leg_delta[8][2] = {{-1, 0}, {-1, 0}, {0, -1}, {0, 1}, {0, -1}, {0, 1}, {1, 0}, {1, 0}};
    for (int site = 0; site < 90; ++site)
    {
        int row = site / 9, col = site % 9;
        masks[site] = BitBoard(site);
        std::vector<Jump> horses, elephants;
        for (int i = 0; i < 8; ++i)
        {
            int r = row + horse_delta[i][0], c = col + horse_delta[i][1];
            if (inside(r, c))
            {
                horses.emplace_back(r * 9 + c, (row + leg_delta[i][0]) * 9 + col + leg_delta[i][1]);
                knight_attacks[site] = knight_attacks[site] | BitBoard(r * 9 + c);
            }
        }
        for (int dr : {-2, 2})
            for (int dc : {-2, 2})
            {
                int r = row + dr, c = col + dc;
                if (inside(r, c) && (row < 5) == (r < 5))
                    elephants.emplace_back(r * 9 + c, (row + dr / 2) * 9 + col + dc / 2);
            }
        jumps(site, horses, knight_legs[site], knight_limited[site], true, &knight_mobility[site]);
        jumps(site, elephants, elephant_legs[site], elephant_limited[site], false, nullptr);
        pawn_checks[site] = knight_legs[site];
        if (pawn_checks[site].count() == 4)
            pawn_checks[site] = pawn_checks[site] ^ BitBoard(site + (site < 45 ? -9 : 9));
        for (int player : {black, red})
        {
            int r = row + (player == black ? 1 : -1);
            if (inside(r, col))
                soldiers[player][site] = soldiers[player][site] | BitBoard(r * 9 + col);
            if ((player == black && row >= 5) || (player == red && row < 5))
                for (int dc : {-1, 1})
                    if (inside(row, col + dc))
                        soldiers[player][site] = soldiers[player][site] | BitBoard(site + dc);
        }
        if (col >= 3 && col <= 5 && (row <= 2 || row >= 7))
        {
            kings[site] = BitBoard();
            for (auto delta : {std::pair<int, int>{-1, 0}, {1, 0}, {0, -1}, {0, 1}})
            {
                int r = row + delta.first, c = col + delta.second;
                if (inside(r, c) && c >= 3 && c <= 5 && ((row <= 2 && r <= 2) || (row >= 7 && r >= 7)))
                    *kings[site] = *kings[site] | BitBoard(r * 9 + c);
            }
            int local = row <= 2 ? row : row - 7;
            if ((local == 1 && col == 4) || (local != 1 && col != 4))
            {
                guards[site] = BitBoard();
                for (int dr : {-1, 1})
                    for (int dc : {-1, 1})
                    {
                        int r = row + dr, c = col + dc;
                        if (inside(r, c) && c >= 3 && c <= 5 && ((row <= 2 && r <= 2) || (row >= 7 && r >= 7)))
                            *guards[site] = *guards[site] | BitBoard(r * 9 + c);
                    }
            }
        }
        ray_tables(site, false, rook_row, quiet_row, cannon_row, cannon_fake_row, cannon_more_row);
        ray_tables(site, true, rook_col, quiet_col, cannon_col, cannon_fake_col, cannon_more_col);
        for (int mask = 0; mask < 512; ++mask)
            row_mobility[site][mask] = quiet_row[site][mask].count();
        for (int mask = 0; mask < 1024; ++mask)
            col_mobility[site][mask] = quiet_col[site][mask].count();
    }
}
const Tables &Tables::get()
{
    static const Tables tables;
    return tables;
}
} // namespace xiangqi
