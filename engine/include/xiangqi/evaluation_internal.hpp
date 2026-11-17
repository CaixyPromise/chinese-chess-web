#pragma once
#include "xiangqi/board.hpp"
namespace xiangqi::evaluation_detail
{
using Pair = std::array<int, 2>;
enum Term
{
    base,
    mobilityPenalty,
    protectMain,
    protectDefense,
    attackMain,
    attackDefense,
    exposedCannon,
    bottomCannon,
    restChariot,
    partition,
    missingElephant,
    missingGuard,
    diversity,
    soldierProtection,
    cannonVsGuards,
    knightVsGuards,
    termCount
};
inline constexpr const char *terms[] = {"base",        "mobility",          "protectMain",     "protectDefense",
                                        "attackMain",  "attackDefense",     "exposedCannon",   "bottomCannon",
                                        "restChariot", "partition",         "missingElephant", "missingGuard",
                                        "diversity",   "soldierProtection", "cannonVsGuards",  "knightVsGuards"};
struct Data
{
    Phase phase, natural_phase;
    std::array<Pair, termCount> term{};
    Pair base{}, recomputedBase{}, residual{}, material{}, positionTotals{}, phaseUnits{}, totals{}, scores{},
        kingImmobile{};
    std::array<int, 48> values{}, position{}, mobility{}, attackWeights{}, defenseWeights{};
    std::array<int, 192> attacks{};
    std::array<int, 14> partitionRaw{};
    std::array<int, 6> attackPartition{}, defensePartition{}, cannonSites{};
    Data(Phase p, Phase n) : phase(p), natural_phase(n)
    {
        mobility.fill(-1);
        cannonSites.fill(-1);
    }
};
int role_type(int role);
int count(const Board &board, int side, int role);
BitBoard all_moves(const Board &board, int role, int site, int side);
int mobility(const Board &board, int role, int site, int side);
const std::array<int, 90> &partition_table(int role);
BitBoard shape(const Board &board, int side, bool main);
Data compute(const Board &board);
int score(const Board &board);
} // namespace xiangqi::evaluation_detail
