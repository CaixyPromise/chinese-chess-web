#include "xiangqi/evaluation_internal.hpp"
#include "xiangqi/legacy_evaluation.hpp"
#include <algorithm>
#include <cmath>
#include <numeric>
#include <sstream>

namespace xiangqi
{
namespace evaluation_detail
{
namespace c = eval_constants::common;
namespace m = eval_constants::middle;
namespace e = eval_constants::end;
using Pair = std::array<int, 2>;

int role_type(int role)
{
    return (role - 1) % 7 + 1;
}
int count(const Board &b, int side, int role)
{
    return b.counts[role_for(side, role)];
}
template <class T> void assign(EvaluationReport &report, const std::string &name, const T &values)
{
    report.fields[name] = std::vector<int>(values.begin(), values.end());
}
BitBoard all_moves(const Board &b, int role, int site, int side)
{
    const auto &t = Tables::get();
    auto row = b.rows[site / 9], col = b.cols[site % 9];
    switch (role_type(role))
    {
    case chariot:
        return t.rook_row[site][row] ^ t.rook_col[site][col] ^ t.quiet_row[site][row] ^ t.quiet_col[site][col];
    case cannon:
        return t.cannon_row[site][row] ^ t.cannon_col[site][col] ^ t.cannon_fake_row[site][row] ^
               t.cannon_fake_col[site][col];
    case knight:
        return t.knight_limited[site][(t.knight_legs[site] & b.occupied).knight_checksum()];
    case elephant:
        return t.elephant_limited[site][(t.elephant_legs[site] & b.occupied).elephant_checksum()];
    case king:
        return t.kings[site].value_or(BitBoard());
    case guard:
        return t.guards[site].value_or(BitBoard());
    case soldier:
        return t.soldiers[side][site];
    default:
        throw std::logic_error("unknown evaluation piece");
    }
}
int mobility(const Board &b, int role, int site, int side)
{
    const auto &t = Tables::get();
    switch (role_type(role))
    {
    case chariot:
    case cannon:
        return t.row_mobility[site][b.rows[site / 9]] + t.col_mobility[site][b.cols[site % 9]];
    case knight:
    {
        auto attacks = all_moves(b, role, site, side);
        return attacks.count() - (attacks & b.personal[side]).count();
    }
    case king:
        return ((*t.kings[site] & b.occupied) ^ *t.kings[site]).count();
    default:
        return -1;
    }
}
const std::array<int, 90> &partition_table(int role)
{
    switch (role_type(role))
    {
    case soldier:
        return c::SoldierPartitionSite;
    case guard:
    case elephant:
        return c::DefensePartitionSite;
    case cannon:
        return c::GunPartitionSite;
    case knight:
        return c::KnightPartitionSite;
    case chariot:
        return c::ChariotPartitionSite;
    default:
        return c::KingPartitionSite;
    }
}
BitBoard shape(const Board &b, int side, bool main)
{
    return main ? b.roles[role_for(side, chariot)] ^ b.roles[role_for(side, knight)] ^ b.roles[role_for(side, cannon)]
                : b.roles[role_for(side, elephant)] ^ b.roles[role_for(side, guard)] ^ b.roles[role_for(side, soldier)];
}
} // namespace evaluation_detail
using namespace evaluation_detail;

Phase natural_phase(const Board &b)
{
    int total = 0;
    for (int side : {black, red})
        total += count(b, side, chariot) + count(b, side, knight) + count(b, side, cannon) +
                 (count(b, side, soldier) > 3 ? 1 : 0);
    return total < 7 ? Phase::end : Phase::middle;
}
std::string phase_name(Phase phase)
{
    return phase == Phase::middle ? "middle" : "end";
}
namespace
{
constexpr int original_position_score(Phase phase, int role, int site)
{
    int type = (role - 1) % 7 + 1;
    bool middle = phase == Phase::middle;
    // Tools.exchange mirrors rows for offensive pieces; defensive tables are shared.
    if (role <= 7 && (type == soldier || type == knight || type == chariot || type == cannon))
        site = (9 - site / 9) * 9 + site % 9;
    switch (type)
    {
    case soldier:
        return middle ? m::blackSoldierAttach[site] : e::blackSoldierAttach[site];
    case guard:
        return middle ? m::GuardAttach[site] : e::GuardAttach[site];
    case elephant:
        return middle ? m::ElephantAttch[site] : e::ElephantAttch[site];
    case cannon:
        return middle ? m::blackGunAttach[site] : e::blackGunAttach[site];
    case knight:
        return middle ? m::blackKnightAttach[site] : e::blackKnightAttach[site];
    case chariot:
        return middle ? m::blackChariotAttach[site] : e::blackChariotAttach[site];
    case king:
        return middle ? m::kingAttach[site] : e::kingAttach[site];
    default:
        throw std::logic_error("invalid position score role");
    }
}
constexpr auto build_position_scores()
{
    std::array<std::array<std::array<int, 90>, 15>, 2> table{};
    for (int phase = 0; phase < 2; ++phase)
        for (int role = 1; role <= 14; ++role)
            for (int site = 0; site < 90; ++site)
                table[phase][role][site] = original_position_score(phase == 0 ? Phase::middle : Phase::end, role, site);
    return table;
}
inline constexpr auto position_scores = build_position_scores();
} // namespace
int position_score(Phase phase, int role, int site)
{
    if (role < 1 || role > 14)
        throw std::logic_error("invalid position score role");
    return position_scores[phase == Phase::middle ? 0 : 1][role][site];
}
Pair recompute_base(const Board &b, const EvaluationState &state)
{
    Pair result{};
    for (int piece = 16; piece < 48; ++piece)
        if (b.pieces[piece] != nothing)
            result[piece_side(piece)] +=
                state.values[piece] + position_score(state.phase, piece_roles[piece], b.pieces[piece]);
    return result;
}
void prepare_evaluation(Board &b, const std::string &phase)
{
    if (phase != "auto" && phase != "middle" && phase != "end")
        throw EngineError("INVALID_ARGUMENT", "evaluation phase must be auto, middle or end");
    EvaluationState state;
    state.phase = phase == "auto" ? natural_phase(b) : phase == "middle" ? Phase::middle : Phase::end;
    state.values = c::chessBaseScore;
    // Preserve cold Java factory ordering: base first, moveBegin dynamic values second.
    state.base = recompute_base(b, state);
    int total = std::accumulate(b.counts.begin(), b.counts.end(), 0);
    for (int piece = 16; piece < 48; ++piece)
    {
        switch (role_type(piece_roles[piece]))
        {
        case soldier:
            state.values[piece] = c::SOLDIERSCORE + (11 - b.attack_defense[piece_side(piece)][0]) * 8;
            break;
        case knight:
            state.values[piece] = c::KNIGHTSCORE + (32 - total) * 6;
            break;
        case cannon:
            state.values[piece] = c::GUNSCORE - (32 - total) * 6;
            break;
        default:
            break;
        }
    }
    auto current = recompute_base(b, state);
    for (int side = 0; side < 2; ++side)
        state.residual[side] = state.base[side] - current[side];
    state.ready = true;
    b.evaluation = state;
}
std::string evaluation_state(const EvaluationState &state)
{
    std::ostringstream out;
    out << state.ready << ':' << phase_name(state.phase) << ':';
    for (auto value : state.values)
        out << value << ',';
    for (auto value : state.base)
        out << value << ',';
    for (auto value : state.residual)
        out << value << ',';
    return out.str();
}

EvaluationReport evaluate(const Board &b)
{
    auto data = evaluation_detail::compute(b);
    EvaluationReport report{data.phase, data.natural_phase, {}};
    for (int term = 0; term < evaluation_detail::termCount; ++term)
        assign(report, std::string("term_") + terms[term], data.term[term]);
    assign(report, "base", data.base);
    assign(report, "values", data.values);
    assign(report, "recomputedBase", data.recomputedBase);
    assign(report, "residual", data.residual);
    assign(report, "position", data.position);
    assign(report, "mobility", data.mobility);
    assign(report, "attacks", data.attacks);
    assign(report, "partitionRaw", data.partitionRaw);
    assign(report, "attackPartition", data.attackPartition);
    assign(report, "defensePartition", data.defensePartition);
    assign(report, "kingImmobile", data.kingImmobile);
    assign(report, "cannonSites", data.cannonSites);
    assign(report, "attackWeights", data.attackWeights);
    assign(report, "defenseWeights", data.defenseWeights);
    assign(report, "material", data.material);
    assign(report, "positionTotals", data.positionTotals);
    assign(report, "phaseUnits", data.phaseUnits);
    assign(report, "totals", data.totals);
    assign(report, "scores", data.scores);
    return report;
}
int evaluate_score(const Board &b)
{
    return evaluation_detail::score(b);
}

std::string evaluation_json(const EvaluationReport &report)
{
    std::ostringstream out;
    out << "{\"phase\":\"" << phase_name(report.phase) << "\",\"naturalPhase\":\"" << phase_name(report.natural_phase)
        << '"';
    for (const auto &field : report.fields)
    {
        out << ",\"" << field.first << "\":[";
        for (std::size_t i = 0; i < field.second.size(); ++i)
        {
            if (i)
                out << ',';
            out << field.second[i];
        }
        out << ']';
    }
    return out.str() + '}';
}
std::string evaluation_tables_json()
{
    EvaluationReport report{Phase::middle, Phase::middle, {}};
    for (auto phase : {Phase::middle, Phase::end})
    {
        auto &values = report.fields[phase_name(phase) + "Pst"];
        for (int role = 1; role <= 14; ++role)
            for (int site = 0; site < 90; ++site)
                values.push_back(position_score(phase, role, site));
    }
    assign(report, "defaultValues", c::chessBaseScore);
    assign(report, "defaultAttack", c::attackChessPartitionScore);
    assign(report, "defaultDefense", c::defenseChessPartitionScore);
    assign(report, "minMobility", m::chessMinMobility);
    assign(report, "mobilityRewards", m::chessMobilityRewards);
    for (int role = 1; role <= 14; ++role)
        for (int site = 0; site < 90; ++site)
            report.fields["partitionSites"].push_back(partition_table(role)[site]);
    assign(report, "soldiersProtected", e::soldiersProtected);
    assign(report, "gunOpptNotGuard", e::gunOpptNotGuard);
    assign(report, "knightOpptNotGuard", e::knightOpptNotGuard);
    return evaluation_json(report);
}
} // namespace xiangqi
