#pragma once

#include <atomic>
#include <cstdint>
#include <functional>
#include <optional>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

namespace xiangqi
{

// a0 is red's bottom-left square; index = (9 - rank) * 9 + file.
struct Move
{
    std::uint8_t from;
    std::uint8_t to;
};

struct Position
{
    std::string initial_fen;
    std::vector<Move> history;
    std::string rules_version = "legacy-java-v1";
    std::string evaluation_phase = "auto"; // Diagnostic root override; search descendants keep the chosen phase.
};

struct SearchLimits
{
    std::uint32_t max_depth = 6;
    std::uint64_t time_ms = 4000;
    std::uint64_t max_nodes = 0; // zero means no node limit; time still applies.
    std::uint32_t hash_mib = 32;
    std::optional<Move> rootMove = std::nullopt; // Restrict root candidates; retain root evaluation/history.
};

struct SearchInfo
{
    std::int32_t score = 0; // side-to-move perspective, legacy score units.
    std::uint32_t completed_depth = 0;
    std::uint64_t nodes = 0;
    std::uint64_t elapsed_ms = 0;
    std::vector<Move> pv;
    std::uint64_t evaluations = 0, cache_hits = 0, cutoffs = 0;
    std::uint64_t completed_ns = 0; // Root completion time, excluding later publication overhead.
};

enum class StopReason
{
    completed,
    deadline,
    cancelled,
    terminal
};
struct SearchResult
{
    bool has_move = false;
    Move best_move{};
    SearchInfo info;
    StopReason reason = StopReason::completed;
};

struct ApplyResult
{
    std::string fen;
    bool in_check = false;
    // Ongoing, red_win, black_win, draw. Rule adjudication is versioned.
    std::string outcome;
};

class EngineError : public std::runtime_error
{
  public:
    EngineError(std::string code, const std::string &message) : std::runtime_error(message), code_(std::move(code))
    {
    }
    const std::string &code() const noexcept
    {
        return code_;
    }

  private:
    std::string code_;
};

// One context per worker, one active computation per context. Read-only lookup
// tables may be shared. Mutable board, evaluation, TT and heuristics must not be.
// set_position is transactional: an error must leave the old context intact.
// search returns only a legal move for the accepted position; interrupted
// iterations must not overwrite the last completed iteration's reliable score.
class Engine
{
  public:
    virtual ~Engine() = default;
    virtual void reset() = 0;
    virtual void set_position(const Position &position) = 0;
    virtual std::vector<Move> legal_moves() = 0;
    virtual ApplyResult apply_move(Move move) = 0;
    virtual SearchResult search(const SearchLimits &limits, const std::atomic_bool &cancelled,
                                const std::function<void(const SearchInfo &)> &on_info) = 0;
};

// Phase-one adapter explicitly refuses chess computations until the port exists.
class UnavailableEngine final : public Engine
{
  public:
    void reset() override;
    void set_position(const Position &) override;
    std::vector<Move> legal_moves() override;
    ApplyResult apply_move(Move) override;
    SearchResult search(const SearchLimits &, const std::atomic_bool &,
                        const std::function<void(const SearchInfo &)> &) override;
};

Move parse_move(const std::string &text);
std::string format_move(Move move);
void validate_fen_structure(const std::string &fen);

} // namespace xiangqi
