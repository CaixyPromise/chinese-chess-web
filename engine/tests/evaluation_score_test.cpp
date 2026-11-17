#include "xiangqi/board.hpp"
#include <iostream>
#include <sstream>
#include <stdexcept>
using namespace xiangqi;
void verify(const Board &board)
{
    auto report = evaluate(board);
    if (evaluate_score(board) != report.fields.at("scores").at(board.side))
        throw std::runtime_error("search score differs from full diagnostic evaluation");
}
int main()
{
    std::string line;
    std::size_t roots = 0, snapshots = 0;
    while (std::getline(std::cin, line))
    {
        std::istringstream row(line);
        std::string phase, fen, moves;
        std::getline(row, phase, '\t');
        std::getline(row, fen, '\t');
        std::getline(row, moves);
        Position position{fen, {}};
        position.evaluation_phase = phase;
        std::istringstream history(moves);
        std::string move;
        while (std::getline(history, move, ','))
            if (!move.empty())
                position.history.push_back(parse_move(move));
        RulesEngine engine;
        engine.set_position(position);
        auto &board = engine.board();
        verify(board);
        ++roots;
        ++snapshots;
        auto before = board.state();
        for (auto candidate : board.legal_moves())
        {
            auto undo = board.make_move(candidate);
            if (board.pieces[16] != nothing && board.pieces[32] != nothing)
            {
                verify(board);
                ++snapshots;
            }
            board.unmake_move(undo);
            if (board.state() != before)
                throw std::runtime_error("evaluation child did not restore root");
        }
    }
    std::cout << roots << " roots and " << snapshots << " search/diagnostic scores matched\n";
}
