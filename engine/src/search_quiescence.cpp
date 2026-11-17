#include "xiangqi/search_internal.hpp"
#include <algorithm>
namespace xiangqi::search_detail
{
int Search::quiet(int alpha, int beta, bool checked, Line &pv)
{
    check();
    ++nodes;
    int distance = ply(), side = board.side;
    if (board.pieces[side == red ? 32 : 16] == nothing)
        return distance - legacy::maxScore;
    if (repeated_check(checked))
        return legacy::LONGCHECKSCORE;
    if (drawn())
        return legacy::drawScore;
    if (distance >= 64)
        return fine();
    bool moved = !checked;
    int best = -legacy::maxScore - 2;
    Line best_line;
    if (!checked)
    {
        int stand = fine();
        if (stand >= beta)
            return stand;
        best = stand;
        alpha = std::max(alpha, stand);
    }
    MovePicker picker(board, heuristics, 0, {}, true, checked);
    while (auto picked = picker.next())
    {
        const auto candidate = *picked;
        check();
        auto undo = board.make_move(candidate.hint.move);
        if (board.checked(side))
        {
            board.unmake_move(undo);
            continue;
        }
        push(candidate.hint);
        Line line;
        int value;
        try
        {
            value = -quiet(-beta, -alpha, board.checked(1 - side), line);
        }
        catch (...)
        {
            pop();
            board.unmake_move(undo);
            throw;
        }
        pop();
        board.unmake_move(undo);
        moved = true;
        if (value > best)
        {
            best = value;
            best_line = {candidate.hint};
            best_line.insert(best_line.end(), line.begin(), line.end());
            alpha = std::max(alpha, value);
            if (value >= beta)
                break;
        }
    }
    if (moved)
    {
        pv = std::move(best_line);
        return best;
    }
    return distance - legacy::maxScore;
}
} // namespace xiangqi::search_detail
