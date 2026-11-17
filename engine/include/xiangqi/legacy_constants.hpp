// Generated from ChessConstant.java; SHA-256: 8c140ae16cd63895ca547e3bfe8ff8499bb1dbb7a0e913a0349635367087f7d6
#pragma once
#include <array>
#include <cstdint>
namespace xiangqi::legacy
{
inline constexpr std::int32_t KING = 7;
inline constexpr std::int32_t CHARIOT = 6;
inline constexpr std::int32_t KNIGHT = 5;
inline constexpr std::int32_t GUN = 4;
inline constexpr std::int32_t ELEPHANT = 3;
inline constexpr std::int32_t GUARD = 2;
inline constexpr std::int32_t SOLDIER = 1;
inline constexpr std::int32_t REDKING = KING + 0;
inline constexpr std::int32_t REDCHARIOT = CHARIOT + 0;
inline constexpr std::int32_t REDKNIGHT = KNIGHT + 0;
inline constexpr std::int32_t REDGUN = GUN + 0;
inline constexpr std::int32_t REDELEPHANT = ELEPHANT + 0;
inline constexpr std::int32_t REDGUARD = GUARD + 0;
inline constexpr std::int32_t REDSOLDIER = SOLDIER + 0;
inline constexpr std::int32_t BLACKKING = KING + 7;
inline constexpr std::int32_t BLACKCHARIOT = CHARIOT + 7;
inline constexpr std::int32_t BLACKKNIGHT = KNIGHT + 7;
inline constexpr std::int32_t BLACKGUN = GUN + 7;
inline constexpr std::int32_t BLACKELEPHANT = ELEPHANT + 7;
inline constexpr std::int32_t BLACKGUARD = GUARD + 7;
inline constexpr std::int32_t BLACKSOLDIER = SOLDIER + 7;
inline constexpr std::int32_t maxScore = 9999;
inline constexpr std::int32_t REDPLAYSIGN = 1;
inline constexpr std::int32_t BLACKPLAYSIGN = 0;
inline constexpr std::int32_t BOARDSIZE90 = 90;
inline constexpr std::int32_t LONGCHECKSCORE = 8888;
inline constexpr std::int32_t drawScore = 0;
inline constexpr std::int32_t NOTHING = -1;
inline constexpr std::int32_t MAXDEPTH = 6;
inline constexpr std::array<int, 48> chessRoles = {{
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    BLACKKING,
    BLACKCHARIOT,
    BLACKCHARIOT,
    BLACKKNIGHT,
    BLACKKNIGHT,
    BLACKGUN,
    BLACKGUN,
    BLACKELEPHANT,
    BLACKELEPHANT,
    BLACKGUARD,
    BLACKGUARD,
    BLACKSOLDIER,
    BLACKSOLDIER,
    BLACKSOLDIER,
    BLACKSOLDIER,
    BLACKSOLDIER,
    REDKING,
    REDCHARIOT,
    REDCHARIOT,
    REDKNIGHT,
    REDKNIGHT,
    REDGUN,
    REDGUN,
    REDELEPHANT,
    REDELEPHANT,
    REDGUARD,
    REDGUARD,
    REDSOLDIER,
    REDSOLDIER,
    REDSOLDIER,
    REDSOLDIER,
    REDSOLDIER,
}};
inline constexpr std::array<int, 48> chessRoles_eight = {{
    0,        0,     0,     0,        0,        0,       0,       0,       0,       0,       0,       0,
    0,        0,     0,     0,        KING,     CHARIOT, CHARIOT, KNIGHT,  KNIGHT,  GUN,     GUN,     ELEPHANT,
    ELEPHANT, GUARD, GUARD, SOLDIER,  SOLDIER,  SOLDIER, SOLDIER, SOLDIER, KING,    CHARIOT, CHARIOT, KNIGHT,
    KNIGHT,   GUN,   GUN,   ELEPHANT, ELEPHANT, GUARD,   GUARD,   SOLDIER, SOLDIER, SOLDIER, SOLDIER, SOLDIER,
}};
inline constexpr std::array<int, 2> chessPlay = {{
    16,
    32,
}};
inline constexpr std::array<int, 90> boardRow = {{
    0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3,
    3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6,
    6, 6, 6, 7, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 8, 9, 9, 9, 9, 9, 9, 9, 9, 9,
}};
inline constexpr std::array<int, 90> boardCol = {{
    0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2,
    3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5,
    6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8,
}};
inline constexpr std::array<int, 256> boardMap = {{
    0, 0, 0, 0,  0,  0,  0,  0,  0,  0,  0,  0,  0, 0, 0, 0, 0, 0, 0, 0,  0,  0,  0,  0,  0,  0,  0,  0,  0, 0, 0, 0,
    0, 0, 0, 0,  0,  0,  0,  0,  0,  0,  0,  0,  0, 0, 0, 0, 0, 0, 0, 0,  1,  2,  3,  4,  5,  6,  7,  8,  0, 0, 0, 0,
    0, 0, 0, 9,  10, 11, 12, 13, 14, 15, 16, 17, 0, 0, 0, 0, 0, 0, 0, 18, 19, 20, 21, 22, 23, 24, 25, 26, 0, 0, 0, 0,
    0, 0, 0, 27, 28, 29, 30, 31, 32, 33, 34, 35, 0, 0, 0, 0, 0, 0, 0, 36, 37, 38, 39, 40, 41, 42, 43, 44, 0, 0, 0, 0,
    0, 0, 0, 45, 46, 47, 48, 49, 50, 51, 52, 53, 0, 0, 0, 0, 0, 0, 0, 54, 55, 56, 57, 58, 59, 60, 61, 62, 0, 0, 0, 0,
    0, 0, 0, 63, 64, 65, 66, 67, 68, 69, 70, 71, 0, 0, 0, 0, 0, 0, 0, 72, 73, 74, 75, 76, 77, 78, 79, 80, 0, 0, 0, 0,
    0, 0, 0, 81, 82, 83, 84, 85, 86, 87, 88, 89, 0, 0, 0, 0, 0, 0, 0, 0,  0,  0,  0,  0,  0,  0,  0,  0,  0, 0, 0, 0,
    0, 0, 0, 0,  0,  0,  0,  0,  0,  0,  0,  0,  0, 0, 0, 0, 0, 0, 0, 0,  0,  0,  0,  0,  0,  0,  0,  0,  0, 0, 0, 0,
}};
} // namespace xiangqi::legacy
