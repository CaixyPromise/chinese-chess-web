#pragma once
#include "xiangqi/engine.hpp"
#include "xiangqi/legacy_constants.hpp"
#include "xiangqi/evaluation.hpp"
#include <array>
#include <optional>

namespace xiangqi
{
inline constexpr int nothing = legacy::NOTHING, black = legacy::BLACKPLAYSIGN, red = legacy::REDPLAYSIGN;
inline constexpr int soldier = legacy::SOLDIER, guard = legacy::GUARD, elephant = legacy::ELEPHANT,
                     cannon = legacy::GUN, knight = legacy::KNIGHT, chariot = legacy::CHARIOT, king = legacy::KING;
inline constexpr auto piece_roles = legacy::chessRoles;
inline int piece_side(int piece)
{
    return piece < 32 ? black : red;
}
inline int role_for(int side, int role)
{
    return role + (side == black ? 7 : 0);
}

struct BitBoard
{
    std::array<std::uint32_t, 4> words{};
    BitBoard() = default;
    explicit BitBoard(int site)
    {
        if (site >= 0 && site < 108)
            words[site / 27] = std::uint32_t{1} << (site % 27);
    }
    BitBoard operator&(const BitBoard &b) const;
    BitBoard operator|(const BitBoard &b) const;
    BitBoard operator^(const BitBoard &b) const;
    bool operator==(const BitBoard &b) const
    {
        return words == b.words;
    }
    bool empty() const;
    bool contains(int site) const;
    int count() const;
    int first(int side) const; // reverse segment order for black, not bits inside segments.
    int knight_checksum() const;
    int elephant_checksum() const;
};

struct Tables
{
    using Grid = std::vector<std::vector<BitBoard>>;
    std::array<BitBoard, 90> masks{}, knight_attacks{}, knight_legs{}, elephant_legs{}, pawn_checks{};
    std::array<std::optional<BitBoard>, 90> kings{}, guards{};
    std::array<std::array<BitBoard, 90>, 2> soldiers{};
    Grid knight_limited, elephant_limited;
    Grid rook_row, rook_col, quiet_row, quiet_col, cannon_row, cannon_col;
    Grid cannon_fake_row, cannon_fake_col, cannon_more_row, cannon_more_col;
    std::array<std::array<int, 200>, 90> knight_mobility{};
    std::array<std::array<int, 512>, 90> row_mobility{};
    std::array<std::array<int, 1024>, 90> col_mobility{};
    Tables();
    static const Tables &get();
};

struct Undo
{
    Move move;
    int moving, captured, previous_side;
    std::uint32_t halfmove, fullmove;
    EvaluationState evaluation;
};

// Mutable state is owned by each board, never stored in process-global variables.
class Board
{
  public:
    Board()
    {
        squares.fill(nothing);
        pieces.fill(nothing);
    }
    std::array<int, 90> squares{};
    std::array<int, 48> pieces{};
    std::array<std::uint32_t, 10> rows{};
    std::array<std::uint32_t, 9> cols{};
    std::array<int, 15> counts{};
    std::array<std::array<int, 2>, 2> attack_defense{};
    BitBoard occupied;
    std::array<BitBoard, 2> personal{};
    std::array<BitBoard, 15> roles{};
    std::uint32_t hash32 = 0;
    std::uint64_t hash64 = 0;
    int side = red;
    std::uint32_t halfmove = 0, fullmove = 1;
    EvaluationState evaluation;

    static Board from_fen(const std::string &fen);
    std::string fen() const;
    bool pseudo_legal(Move move) const;
    bool checked(int player) const;
    std::vector<Move> legal_moves();
    Undo make_move(Move move); // Validated indices and own piece required; geometry checked by caller.
    void unmake_move(const Undo &undo);
    std::uint64_t perft(int depth);
    bool consistent() const;
    std::string core_state() const;
    std::string state() const; // Canonical full state for restoration tests, not a position identifier.
  private:
    void put(int site, int piece);
};

class RulesEngine final : public Engine
{
  public:
    void reset() override;
    void set_position(const Position &) override;
    std::vector<Move> legal_moves() override;
    ApplyResult apply_move(Move) override;
    SearchResult search(const SearchLimits &, const std::atomic_bool &,
                        const std::function<void(const SearchInfo &)> &) override;
    Board &board();
    void undo();

  private:
    std::optional<Board> board_;
    std::vector<Undo> history_;
    std::string evaluation_phase_ = "auto";
};
} // namespace xiangqi
