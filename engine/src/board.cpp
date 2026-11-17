#include "xiangqi/board.hpp"
#include "xiangqi/legacy_zobrist.hpp"
#include <algorithm>
#include <sstream>

namespace xiangqi
{
namespace
{
constexpr std::array<int, 15> defense_index = {0, 0, 1, 1, 0, 0, 0, 1, 0, 1, 1, 0, 0, 0, 1};
char symbol(int piece)
{
    constexpr char roles[] = " pabcnrk";
    char c = roles[(piece_roles[piece] - 1) % 7 + 1];
    return piece_side(piece) == red ? static_cast<char>(c - 'a' + 'A') : c;
}
bool same_move(Move a, Move b)
{
    return a.from == b.from && a.to == b.to;
}
} // namespace

void Board::put(int site, int piece)
{
    const auto &t = Tables::get();
    int old = squares[site];
    for (int value : {old, piece})
    {
        if (value == nothing)
            continue;
        int role = piece_roles[value], player = piece_side(value);
        occupied = occupied ^ t.masks[site];
        personal[player] = personal[player] ^ t.masks[site];
        roles[role] = roles[role] ^ t.masks[site];
        rows[site / 9] ^= std::uint32_t{1} << (8 - site % 9);
        cols[site % 9] ^= std::uint32_t{1} << (9 - site / 9);
        hash32 ^= zobrist32[site][role];
        hash64 ^= zobrist64[site][role];
    }
    if (old != nothing)
    {
        if (evaluation.ready)
            evaluation.base[piece_side(old)] -=
                evaluation.values[old] + position_score(evaluation.phase, piece_roles[old], site);
        --counts[piece_roles[old]];
        --attack_defense[piece_side(old)][defense_index[piece_roles[old]]];
        pieces[old] = nothing;
    }
    if (piece != nothing)
    {
        if (evaluation.ready)
            evaluation.base[piece_side(piece)] +=
                evaluation.values[piece] + position_score(evaluation.phase, piece_roles[piece], site);
        ++counts[piece_roles[piece]];
        ++attack_defense[piece_side(piece)][defense_index[piece_roles[piece]]];
        pieces[piece] = site;
    }
    squares[site] = piece;
}

Board Board::from_fen(const std::string &fen)
{
    validate_fen_structure(fen);
    Board board;
    board.squares.fill(nothing);
    board.pieces.fill(nothing);
    std::istringstream input(fen);
    std::string layout, side, dash;
    input >> layout >> side >> dash >> dash >> board.halfmove >> board.fullmove;
    board.side = side == "r" ? red : black;
    std::array<int, 128> next{};
    next['k'] = 16;
    next['r'] = 17;
    next['n'] = 19;
    next['c'] = 21;
    next['b'] = 23;
    next['a'] = 25;
    next['p'] = 27;
    next['K'] = 32;
    next['R'] = 33;
    next['N'] = 35;
    next['C'] = 37;
    next['B'] = 39;
    next['A'] = 41;
    next['P'] = 43;
    int site = 0;
    const auto &tables = Tables::get();
    for (unsigned char c : layout)
    {
        if (c == '/')
            continue;
        if (c >= '1' && c <= '9')
        {
            site += c - '0';
            continue;
        }
        int piece = next[c]++;
        int role = (piece_roles[piece] - 1) % 7 + 1;
        // Java dereferences null tables for off-domain kings/advisors. Reject
        // these unsupported setups explicitly instead of crashing the worker.
        int row = site / 9;
        bool own_palace = piece_side(piece) == black ? row <= 2 : row >= 7;
        if ((role == king && (!own_palace || !tables.kings[site])) ||
            (role == guard && (!own_palace || !tables.guards[site])))
            throw EngineError("INVALID_POSITION", "king or advisor is outside its supported palace squares");
        board.put(site++, piece);
    }
    prepare_evaluation(board);
    return board;
}

std::string Board::fen() const
{
    std::ostringstream out;
    for (int row = 0; row < 10; ++row)
    {
        if (row)
            out << '/';
        int empty = 0;
        for (int col = 0; col < 9; ++col)
        {
            int piece = squares[row * 9 + col];
            if (piece == nothing)
            {
                ++empty;
                continue;
            }
            if (empty)
            {
                out << empty;
                empty = 0;
            }
            out << symbol(piece);
        }
        if (empty)
            out << empty;
    }
    out << (side == red ? " r - - " : " b - - ") << halfmove << ' ' << fullmove;
    return out.str();
}

bool Board::pseudo_legal(Move move) const
{
    if (move.from >= 90 || move.to >= 90 || move.from == move.to)
        return false;
    int moving = squares[move.from], captured = squares[move.to];
    if (moving == nothing || piece_side(moving) != side || (captured != nothing && piece_side(captured) == side))
        return false;
    const auto &t = Tables::get();
    int src = move.from;
    auto row = rows[src / 9], col = cols[src % 9];
    BitBoard attacks;
    switch ((piece_roles[moving] - 1) % 7 + 1)
    {
    case chariot:
        attacks = captured == nothing ? t.quiet_row[src][row] ^ t.quiet_col[src][col]
                                      : t.rook_row[src][row] ^ t.rook_col[src][col];
        break;
    case cannon:
        attacks = captured == nothing ? t.quiet_row[src][row] ^ t.quiet_col[src][col]
                                      : t.cannon_row[src][row] ^ t.cannon_col[src][col];
        break;
    case knight:
        attacks = t.knight_limited[src][(t.knight_legs[src] & occupied).knight_checksum()];
        break;
    case elephant:
        attacks = t.elephant_limited[src][(t.elephant_legs[src] & occupied).elephant_checksum()];
        break;
    case king:
        if (t.kings[src])
            attacks = *t.kings[src];
        break;
    case guard:
        if (t.guards[src])
            attacks = *t.guards[src];
        break;
    case soldier:
        attacks = t.soldiers[side][src];
        break;
    default:
        return false;
    }
    return attacks.contains(move.to);
}

bool Board::checked(int player) const
{
    int opponent = 1 - player;
    int theirs = pieces[opponent == black ? 16 : 32];
    if (theirs == nothing)
        return false; // Preserve the Java capture-king convention.
    int ours = pieces[player == black ? 16 : 32];
    if (ours == nothing)
        return true;
    const auto &t = Tables::get();
    auto row = rows[ours / 9], col = cols[ours % 9];
    auto rook = t.rook_row[ours][row] ^ t.rook_col[ours][col];
    if (!(rook & roles[role_for(opponent, chariot)]).empty())
        return true;
    if (t.rook_col[ours][col].contains(theirs))
        return true;
    auto gun = t.cannon_row[ours][row] ^ t.cannon_col[ours][col];
    if (!(gun & roles[role_for(opponent, cannon)]).empty())
        return true;
    int first_knight = opponent == black ? 19 : 35;
    for (int piece = first_knight; piece < first_knight + 2; ++piece)
    {
        int src = pieces[piece];
        if (src != nothing && t.knight_attacks[ours].contains(src) &&
            t.knight_limited[src][(t.knight_legs[src] & occupied).knight_checksum()].contains(ours))
            return true;
    }
    return !(t.pawn_checks[ours] & roles[role_for(opponent, soldier)]).empty();
}

Undo Board::make_move(Move move)
{
    if (move.from >= 90 || move.to >= 90 || move.from == move.to || squares[move.from] == nothing ||
        piece_side(squares[move.from]) != side || (squares[move.to] != nothing && piece_side(squares[move.to]) == side))
        throw EngineError("ILLEGAL_MOVE", "invalid move source or destination");
    Undo undo{move, squares[move.from], squares[move.to], side, halfmove, fullmove, evaluation};
    put(move.from, nothing);
    put(move.to, undo.moving);
    halfmove = undo.captured != nothing || (piece_roles[undo.moving] - 1) % 7 + 1 == soldier ? 0 : halfmove + 1;
    if (side == black)
        ++fullmove;
    side = 1 - side;
    return undo;
}

void Board::unmake_move(const Undo &undo)
{
    put(undo.move.to, undo.captured);
    put(undo.move.from, undo.moving);
    side = undo.previous_side;
    halfmove = undo.halfmove;
    fullmove = undo.fullmove;
    evaluation = undo.evaluation;
}

std::vector<Move> Board::legal_moves()
{
    std::vector<Move> result;
    if (pieces[side == black ? 16 : 32] == nothing)
        return result;
    for (int from = 0; from < 90; ++from)
    {
        if (squares[from] == nothing || piece_side(squares[from]) != side)
            continue;
        for (int to = 0; to < 90; ++to)
        {
            Move move{static_cast<std::uint8_t>(from), static_cast<std::uint8_t>(to)};
            if (!pseudo_legal(move))
                continue;
            auto undo = make_move(move);
            bool safe = !checked(undo.previous_side);
            unmake_move(undo);
            if (safe)
                result.push_back(move);
        }
    }
    return result;
}

std::uint64_t Board::perft(int depth)
{
    if (depth < 0 || depth > 6)
        throw EngineError("INVALID_ARGUMENT", "perft depth must be 0..6");
    if (depth == 0)
        return 1;
    std::uint64_t nodes = 0;
    for (Move move : legal_moves())
    {
        auto undo = make_move(move);
        nodes += perft(depth - 1);
        unmake_move(undo);
    }
    return nodes;
}

std::string Board::core_state() const
{
    std::ostringstream out;
    auto array = [&out](const auto &values)
    {
        for (auto value : values)
            out << value << ',';
        out << ';';
    };
    array(squares);
    array(pieces);
    array(rows);
    array(cols);
    array(counts);
    for (auto values : attack_defense)
        array(values);
    array(occupied.words);
    for (auto bits : personal)
        array(bits.words);
    for (auto bits : roles)
        array(bits.words);
    return out.str();
}

std::string Board::state() const
{
    return core_state() + std::to_string(side) + ':' + std::to_string(halfmove) + ':' + std::to_string(fullmove) + ':' +
           std::to_string(hash32) + ':' + std::to_string(hash64) + ':' + evaluation_state(evaluation);
}

bool Board::consistent() const
{
    Board rebuilt;
    rebuilt.squares.fill(nothing);
    rebuilt.pieces.fill(nothing);
    rebuilt.side = side;
    rebuilt.halfmove = halfmove;
    rebuilt.fullmove = fullmove;
    for (int site = 0; site < 90; ++site)
    {
        int piece = squares[site];
        if (piece != nothing)
        {
            if (piece < 16 || piece >= 48 || rebuilt.pieces[piece] != nothing)
                return false;
            rebuilt.put(site, piece);
        }
    }
    rebuilt.evaluation = evaluation;
    if (evaluation.ready)
    {
        auto base = recompute_base(rebuilt, evaluation);
        for (int player = 0; player < 2; ++player)
            if (evaluation.base[player] != base[player] + evaluation.residual[player])
                return false;
    }
    return state() == rebuilt.state();
}

void RulesEngine::reset()
{
    board_.reset();
    history_.clear();
    evaluation_phase_ = "auto";
}
Board &RulesEngine::board()
{
    if (!board_)
        throw EngineError("NO_POSITION", "set position before chess operations");
    return *board_;
}
void RulesEngine::set_position(const Position &position)
{
    if (position.rules_version != "legacy-java-v1")
        throw EngineError("UNSUPPORTED_RULES", "unknown rules version");
    Board next = Board::from_fen(position.initial_fen);
    std::vector<Undo> history;
    for (Move move : position.history)
    {
        auto legal = next.legal_moves();
        if (std::none_of(legal.begin(), legal.end(), [move](Move m) { return same_move(m, move); }))
            throw EngineError("ILLEGAL_MOVE", "illegal history move");
        history.push_back(next.make_move(move));
    }
    // As in the Java baseline, replay history first, then initialize this root.
    prepare_evaluation(next, position.evaluation_phase);
    board_ = std::move(next);
    history_ = std::move(history);
    evaluation_phase_ = position.evaluation_phase;
}
std::vector<Move> RulesEngine::legal_moves()
{
    return board().legal_moves();
}
ApplyResult RulesEngine::apply_move(Move move)
{
    auto legal = legal_moves();
    if (std::none_of(legal.begin(), legal.end(), [move](Move m) { return same_move(m, move); }))
        throw EngineError("ILLEGAL_MOVE", "move is not legal");
    history_.push_back(board().make_move(move));
    auto next_legal = board().legal_moves();
    std::string outcome = next_legal.empty() ? (board().side == red ? "black_win" : "red_win") : "ongoing";
    return {board().fen(), board().checked(board().side), outcome};
}
void RulesEngine::undo()
{
    if (history_.empty())
        throw EngineError("NO_HISTORY", "no move to undo");
    board().unmake_move(history_.back());
    history_.pop_back();
}
} // namespace xiangqi
