#include "xiangqi/protocol.hpp"
#include <algorithm>
#include <limits>

namespace xiangqi::wire
{
bool token(const std::string &value)
{
    return !value.empty() && value.size() <= 128 &&
           value.find_first_not_of("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.:-") ==
               std::string::npos;
}

std::vector<std::string> split(const std::string &input, char delimiter)
{
    std::vector<std::string> result;
    std::size_t start = 0;
    while (true)
    {
        auto end = input.find(delimiter, start);
        result.push_back(input.substr(start, end == std::string::npos ? end : end - start));
        if (end == std::string::npos)
            break;
        start = end + 1;
    }
    return result;
}

void schema(const Fields &fields, const std::set<std::string> &allowed, const std::set<std::string> &required)
{
    for (const auto &field : fields)
        if (!allowed.count(field.first))
            throw xiangqi::EngineError("INVALID_ARGUMENT", "unknown field");
    for (const auto &name : required)
        if (!fields.count(name) || fields.at(name).empty())
            throw xiangqi::EngineError("INVALID_ARGUMENT", "missing required field");
}

std::uint64_t number(const std::string &text, std::uint64_t minimum, std::uint64_t maximum)
{
    if (text.empty() || text.size() > 20 || text.find_first_not_of("0123456789") != std::string::npos)
        throw xiangqi::EngineError("INVALID_ARGUMENT", "expected unsigned decimal number");
    std::uint64_t value = 0;
    for (char c : text)
    {
        auto digit = static_cast<std::uint64_t>(c - '0');
        if (value > (std::numeric_limits<std::uint64_t>::max() - digit) / 10)
            throw xiangqi::EngineError("INVALID_ARGUMENT", "number overflow");
        value = value * 10 + digit;
    }
    if (value < minimum || value > maximum)
        throw xiangqi::EngineError("INVALID_ARGUMENT", "number outside supported range");
    return value;
}

std::string list_moves(std::vector<xiangqi::Move> moves)
{
    std::vector<std::string> text;
    for (auto move : moves)
        text.push_back(xiangqi::format_move(move));
    std::sort(text.begin(), text.end());
    std::string out;
    for (const auto &move : text)
    {
        if (!out.empty())
            out += ',';
        out += move;
    }
    return out;
}

} // namespace xiangqi::wire
