#pragma once
#include <array>
#include <map>
#include <string>
#include <vector>

namespace xiangqi
{
class Board;
enum class Phase
{
    middle,
    end
};
struct EvaluationState
{
    bool ready = false;
    Phase phase = Phase::middle;
    std::array<int, 48> values{};
    std::array<int, 2> base{}, residual{};
};
struct EvaluationReport
{
    Phase phase, natural_phase;
    std::map<std::string, std::vector<int>> fields;
};
Phase natural_phase(const Board &board);
std::string phase_name(Phase phase);
int position_score(Phase phase, int role, int site);
std::array<int, 2> recompute_base(const Board &board, const EvaluationState &state);
void prepare_evaluation(Board &board, const std::string &phase = "auto");
EvaluationReport evaluate(const Board &board);
int evaluate_score(const Board &board);
std::string evaluation_json(const EvaluationReport &report);
std::string evaluation_tables_json();
std::string evaluation_state(const EvaluationState &state);
} // namespace xiangqi
