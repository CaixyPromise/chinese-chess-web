#include "xiangqi/engine.hpp"

#include <array>
#include <sstream>
#include <utility>

namespace xiangqi
{
namespace
{
[[noreturn]] void unavailable()
{
    throw EngineError("NOT_IMPLEMENTED", "C++ chess algorithms are not ported yet");
}

int square(char file, char rank)
{
    if (file < 'a' || file > 'i' || rank < '0' || rank > '9')
        throw EngineError("INVALID_ARGUMENT", "move must use a0..i9 coordinates");
    return (9 - (rank - '0')) * 9 + file - 'a';
}
} // namespace

Move parse_move(const std::string &text)
{
    if (text.size() != 4)
        throw EngineError("INVALID_ARGUMENT", "move must contain four characters");
    auto from = static_cast<std::uint8_t>(square(text[0], text[1]));
    auto to = static_cast<std::uint8_t>(square(text[2], text[3]));
    if (from == to)
        throw EngineError("INVALID_ARGUMENT", "move squares must differ");
    return {from, to};
}

std::string format_move(Move move)
{
    if (move.from >= 90 || move.to >= 90 || move.from == move.to)
        throw EngineError("INVALID_ARGUMENT", "invalid move indices");
    std::string text;
    for (auto site : {move.from, move.to})
    {
        text += static_cast<char>('a' + site % 9);
        text += static_cast<char>('0' + 9 - site / 9);
    }
    return text;
}

void validate_fen_structure(const std::string &fen)
{
    std::istringstream input(fen);
    std::array<std::string, 6> fields;
    std::string extra;
    for (auto &field : fields)
        if (!(input >> field))
            throw EngineError("INVALID_ARGUMENT", "FEN requires six fields");
    if (input >> extra || (fields[1] != "r" && fields[1] != "b") || fields[2] != "-" || fields[3] != "-")
        throw EngineError("INVALID_ARGUMENT", "unsupported FEN fields");
    for (int index : {4, 5})
    {
        const auto &counter = fields[static_cast<std::size_t>(index)];
        if (counter.empty() || counter.size() > 9 || counter.find_first_not_of("0123456789") != std::string::npos ||
            (index == 5 && counter.find_first_not_of('0') == std::string::npos))
            throw EngineError("INVALID_ARGUMENT", "invalid FEN counter");
    }
    int ranks = 1;
    int width = 0;
    std::array<int, 128> counts{};
    const std::string pieces = "rnbakcpRNBAKCP";
    for (unsigned char c : fields[0])
    {
        if (c == '/')
        {
            if (width != 9)
                throw EngineError("INVALID_ARGUMENT", "FEN rank width must be nine");
            ++ranks;
            width = 0;
        }
        else if (c >= '1' && c <= '9')
        {
            width += c - '0';
        }
        else if (pieces.find(static_cast<char>(c)) != std::string::npos)
        {
            ++width;
            int limit = c == 'k' || c == 'K' ? 1 : c == 'p' || c == 'P' ? 5 : 2;
            if (++counts[c] > limit)
                throw EngineError("INVALID_ARGUMENT", "too many FEN pieces");
        }
        else
            throw EngineError("INVALID_ARGUMENT", "invalid FEN piece");
        if (width > 9)
            throw EngineError("INVALID_ARGUMENT", "FEN rank too wide");
    }
    if (ranks != 10 || width != 9 || counts['k'] != 1 || counts['K'] != 1)
        throw EngineError("INVALID_ARGUMENT", "FEN needs ten ranks and both kings");
}

void UnavailableEngine::reset()
{
}
void UnavailableEngine::set_position(const Position &)
{
    unavailable();
}
std::vector<Move> UnavailableEngine::legal_moves()
{
    unavailable();
}
ApplyResult UnavailableEngine::apply_move(Move)
{
    unavailable();
}
SearchResult UnavailableEngine::search(const SearchLimits &, const std::atomic_bool &,
                                       const std::function<void(const SearchInfo &)> &)
{
    unavailable();
}
} // namespace xiangqi
