#include "xiangqi/evaluation_internal.hpp"
#include "xiangqi/legacy_evaluation.hpp"
#include <algorithm>
#include <cmath>
#include <type_traits>
namespace xiangqi::evaluation_detail
{
namespace c = eval_constants::common;
namespace m = eval_constants::middle;
namespace e = eval_constants::end;
template <bool Diagnostic> std::conditional_t<Diagnostic, Data, int> compute_impl(const Board &b)
{
    if (!b.evaluation.ready)
        throw EngineError("NO_EVALUATION", "initialize evaluation context first");
    if (b.pieces[16] == nothing || b.pieces[32] == nothing)
        throw EngineError("TERMINAL_POSITION", "evaluation requires both kings; terminal scoring belongs to search");
    Data report(b.evaluation.phase, Diagnostic ? natural_phase(b) : b.evaluation.phase);
    report.base = b.evaluation.base;
    if constexpr (Diagnostic)
    {
        report.values = b.evaluation.values;
        report.recomputedBase = recompute_base(b, b.evaluation);
        report.residual = b.evaluation.residual;
    }
    report.term[base] = report.base;
    report.kingImmobile = {0, 0};
    auto attack_weights = c::attackChessPartitionScore, defense_weights = c::defenseChessPartitionScore;
    bool middle = b.evaluation.phase == Phase::middle;
    if (middle)
        for (int side : {black, red})
        {
            int base = side == black ? 16 : 32;
            for (int p : {base + 7, base + 8})
                defense_weights[p] = c::guarAndElephantNumScore[count(b, side, elephant)];
            for (int p : {base + 9, base + 10})
                defense_weights[p] = c::guarAndElephantNumScore[count(b, side, guard)];
            for (int p : {base + 5, base + 6})
                attack_weights[p] = c::gunNumScoreDependGuard[count(b, 1 - side, guard)];
            for (int p : {base + 3, base + 4})
                attack_weights[p] = c::knightNumScoreDependGuard[count(b, 1 - side, guard)];
        }
    if constexpr (Diagnostic)
    {
        report.attackWeights = attack_weights;
        report.defenseWeights = defense_weights;
    }
    std::array<BitBoard, 2> attacks{}, soldier_attacks{};
    Pair material{}, position_totals{};
    for (int piece = 16; piece < 48; ++piece)
    {
        int site = b.pieces[piece];
        if (site == nothing)
            continue;
        int side = piece_side(piece), role = piece_roles[piece];
        if constexpr (!Diagnostic)
            if (!middle && role_type(role) != soldier)
                continue;
        auto move = all_moves(b, role, site, side);
        attacks[side] = attacks[side] | move;
        if (role_type(role) == soldier)
            soldier_attacks[side] = soldier_attacks[side] | move;
        if constexpr (Diagnostic)
        {
            report.position[piece] = position_score(b.evaluation.phase, role, site);
            material[side] += b.evaluation.values[piece];
            position_totals[side] += report.position[piece];
            for (int word = 0; word < 4; ++word)
                report.attacks[piece * 4 + word] = static_cast<int>(move.words[word]);
        }
        if ((Diagnostic || middle) && m::chessMinMobility[piece] > 0)
        {
            int value = mobility(b, role, site, side);
            if constexpr (Diagnostic)
                report.mobility[piece] = value;
            if (middle && value < m::chessMinMobility[piece])
            {
                report.term[mobilityPenalty][side] -=
                    (m::chessMinMobility[piece] - value) * m::chessMobilityRewards[piece];
                if (role_type(role) == king)
                    report.kingImmobile[side] = 1;
            }
        }
        if (!middle)
            continue;
        int region = partition_table(role)[site];
        bool top = region == 1 || region == 2 || region == 3 || region == 31 || region == 32 || region == 33;
        int weight = (side == red) == top ? attack_weights[piece] : defense_weights[piece];
        auto add = [&](int zone) { report.partitionRaw[side * 7 + zone] += weight; };
        switch (region)
        {
        case 1:
        case 2:
        case 3:
        case 4:
        case 5:
        case 6:
            add(region);
            break;
        case 31:
            add(3);
            add(1);
            break;
        case 32:
            add(3);
            add(2);
            break;
        case 33:
            add(3);
            add(2);
            add(1);
            break;
        case 64:
            add(6);
            add(4);
            break;
        case 65:
            add(6);
            add(5);
            break;
        case 66:
            add(6);
            add(5);
            add(4);
            break;
        default:
            break;
        }
    }
    if constexpr (Diagnostic)
    {
        report.material = material;
        report.positionTotals = position_totals;
        Pair phase_units{};
        for (int side : {black, red})
            phase_units[side] = count(b, side, chariot) + count(b, side, knight) + count(b, side, cannon) +
                                (count(b, side, soldier) > 3 ? 1 : 0);
        report.phaseUnits = phase_units;
    }
    if (middle)
    {
        for (int side : {black, red})
            for (int region = 0; region < 3; ++region)
            {
                report.attackPartition[side * 3 + region] =
                    report.partitionRaw[side * 7 + (side == red ? 1 : 4) + region];
                report.defensePartition[side * 3 + region] =
                    report.partitionRaw[side * 7 + (side == red ? 4 : 1) + region];
            }
        const auto &t = Tables::get();
        for (int side : {black, red})
        {
            int opp = 1 - side;
            report.term[protectMain][side] = (attacks[side] & shape(b, side, true)).count() * 10;
            report.term[protectDefense][side] = (attacks[side] & shape(b, side, false)).count() * 6;
            report.term[attackMain][side] = (attacks[side] & shape(b, opp, true)).count() * 18;
            report.term[attackDefense][side] = (attacks[side] & shape(b, opp, false)).count() * 9;
            int king_site = b.pieces[opp == black ? 16 : 32];
            auto row = b.rows[king_site / 9], col = b.cols[king_site % 9];
            bool weakness = false;
            int guns = count(b, side, cannon);
            if (guns > 0)
            {
                int exposed =
                    ((t.rook_row[king_site][row] ^ t.rook_col[king_site][col]) & b.roles[role_for(side, cannon)])
                        .first(side);
                // Java's short-circuit leaves exposedCannon uncalled for <=5 opponent pieces.
                if (b.attack_defense[opp][0] + b.attack_defense[opp][1] - 1 > 5)
                {
                    report.cannonSites[side * 3] = exposed;
                    if (exposed != -1)
                    {
                        report.term[exposedCannon][side] =
                            std::abs(king_site / 9 - exposed / 9 + king_site % 9 - exposed % 9) * 45;
                        weakness = true;
                    }
                }
                int bottom = ((t.cannon_more_row[king_site][row] ^ t.cannon_more_col[king_site][col]) &
                              b.roles[role_for(side, cannon)])
                                 .first(side);
                report.cannonSites[side * 3 + 1] = bottom;
                if (bottom != -1 && std::abs(king_site / 9 - bottom / 9 + king_site % 9 - bottom % 9) <= 3)
                {
                    report.term[bottomCannon][side] = 100;
                    weakness = true;
                }
            }
            int rest =
                ((t.cannon_row[king_site][row] ^ t.cannon_col[king_site][col]) & b.roles[role_for(side, chariot)])
                    .first(side);
            report.cannonSites[side * 3 + 2] = rest;
            if (rest != 1)
                report.term[restChariot][side] = 30; // Literal legacy condition.
            for (int region = 0; region < 3; ++region)
            {
                int &defense = report.defensePartition[opp * 3 + region];
                if (weakness)
                    --defense;
                if (report.kingImmobile[opp])
                    defense -= weakness ? 2 : 1;
            }
            if (king_site % 9 == 3)
                --report.defensePartition[opp * 3];
            else if (king_site % 9 == 5)
                --report.defensePartition[opp * 3 + 1];
            else if (king_site % 9 == 4 &&
                     (king_site / 9 == 1 || king_site / 9 == 2 || king_site / 9 == 7 || king_site / 9 == 8))
                --report.defensePartition[opp * 3 + 2];
            for (int region = 0; region < 3; ++region)
                report.term[partition][side] +=
                    std::max(0, report.attackPartition[side * 3 + region] - report.defensePartition[opp * 3 + region]) *
                    30;
            if (count(b, opp, elephant) < 2 && count(b, opp, guard) >= 2 && guns > 0)
                report.term[missingElephant][side] = 60;
            if (count(b, opp, guard) < 2 && count(b, side, knight) > 0)
                report.term[missingGuard][side] = 60;
            for (int role : {chariot, knight, cannon})
                if (count(b, side, role) > 0)
                    report.term[diversity][side] += 100;
        }
    }
    else
    {
        for (int side : {black, red})
        {
            int running = b.evaluation.base[side];
            auto reward = [&](Term name, int value)
            {
                report.term[name][side] = value;
                running += value;
            };
            if (count(b, side, soldier) >= 2)
                reward(soldierProtection,
                       e::soldiersProtected[(soldier_attacks[side] & b.roles[role_for(side, soldier)]).count()]);
            for (int role : {cannon, knight})
            {
                int amount = count(b, side, role);
                if (!amount)
                    continue;
                int value = role == cannon ? e::gunOpptNotGuard[count(b, 1 - side, guard)]
                                           : e::knightOpptNotGuard[count(b, 1 - side, guard)];
                // Java compound += narrows the sum after floating-point multiplication.
                int after = static_cast<int>(static_cast<double>(running) + value * (amount == 2 ? 1.7 : 1.0));
                reward(role == cannon ? cannonVsGuards : knightVsGuards, after - running);
            }
        }
    }
    Pair totals{};
    for (const auto &term : report.term)
        for (int side = 0; side < 2; ++side)
            totals[side] += term[side];
    report.totals = totals;
    report.scores = {totals[0] - totals[1], totals[1] - totals[0]};
    if constexpr (Diagnostic)
        return report;
    else
        return report.scores[b.side];
}

Data compute(const Board &b)
{
    return compute_impl<true>(b);
}
int score(const Board &b)
{
    return compute_impl<false>(b);
}

} // namespace xiangqi::evaluation_detail
