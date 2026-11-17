#include "xiangqi/protocol.hpp"
#include <limits>

namespace xiangqi::wire
{
bool Session::handle(const std::string &line)
{
    std::string id = "-";
    try
    {
        auto parts = split(line, '\t');
        if (parts.size() >= 2 && token(parts[1]))
            id = parts[1];
        for (unsigned char c : line)
            if (c != '\t' && (c < 32 || c > 126))
                throw xiangqi::EngineError("INVALID_ARGUMENT", "wire format is printable ASCII with tab separators");
        if (parts.size() < 3 || !token(parts[1]) || !token(parts[2]))
            throw xiangqi::EngineError("INVALID_ARGUMENT", "invalid request envelope");
        if (parts[0] != "xq1")
            throw xiangqi::EngineError("UNSUPPORTED_VERSION", "expected xq1");
        Fields fields;
        for (std::size_t i = 3; i < parts.size(); ++i)
        {
            auto equal = parts[i].find('=');
            if (equal == std::string::npos || !token(parts[i].substr(0, equal)) ||
                !fields.emplace(parts[i].substr(0, equal), parts[i].substr(equal + 1)).second)
                throw xiangqi::EngineError("INVALID_ARGUMENT", "invalid or duplicate field");
        }
        join_finished();
        const auto &command = parts[2];
        if (job_ && !job_->done.load(std::memory_order_acquire) && command != "stop" && command != "ping" &&
            command != "hello" && command != "quit")
            throw EngineError("BUSY", "search is active");
        if (command == "hello")
        {
            schema(fields, {});
            emit(id, "ok",
                 "engine=cpp-native\tengine_version=0.5.2\tprotocol=xq1\tchess_ready=true\trules_ready="
                 "true\tevaluation_ready=true\tsearch_ready=true\troot_move_ready=true\tstrength_verified=false\trules_"
                 "complete=false\tcommands=hello,ping,reset,quit,position,legal,apply,undo,inspect,perft,evaluate,eval-"
                 "tables,search,stop");
        }
        else if (command == "ping")
        {
            schema(fields, {});
            emit(id, "ok", "pong=true");
        }
        else if (command == "reset")
        {
            schema(fields, {});
            engine_.reset();
            emit(id, "ok", "reset=true");
        }
        else if (command == "quit")
        {
            schema(fields, {});
            shutdown();
            emit(id, "ok", "bye=true");
            return false;
        }
        else if (command == "position")
        {
            schema(fields, {"fen", "moves", "rules", "phase"}, {"fen", "rules"});
            xiangqi::validate_fen_structure(fields.at("fen"));
            if (fields.at("rules") != "legacy-java-v1")
                throw xiangqi::EngineError("UNSUPPORTED_RULES", "unknown rules version");
            xiangqi::Position position;
            position.initial_fen = fields.at("fen");
            position.rules_version = fields.at("rules");
            if (fields.count("phase"))
                position.evaluation_phase = fields.at("phase");
            if (fields.count("moves") && !fields.at("moves").empty())
                for (const auto &move : split(fields.at("moves"), ','))
                    position.history.push_back(xiangqi::parse_move(move));
            engine_.set_position(position);
            emit(id, "ok", "position_accepted=true");
        }
        else if (command == "legal")
        {
            schema(fields, {});
            auto moves = engine_.legal_moves();
            emit(id, "ok",
                 "moves=" + list_moves(moves) +
                     "\tin_check=" + (engine_.board().checked(engine_.board().side) ? "true" : "false"));
        }
        else if (command == "apply")
        {
            schema(fields, {"move"}, {"move"});
            auto result = engine_.apply_move(xiangqi::parse_move(fields.at("move")));
            emit(id, "ok",
                 "fen=" + result.fen + "\tin_check=" + (result.in_check ? "true" : "false") +
                     "\toutcome=" + result.outcome + "\trules_complete=false");
        }
        else if (command == "undo")
        {
            schema(fields, {});
            engine_.undo();
            emit(id, "ok", "fen=" + engine_.board().fen());
        }
        else if (command == "inspect")
        {
            schema(fields, {});
            auto &board = engine_.board();
            emit(id, "ok",
                 "fen=" + board.fen() + "\thash32=" + std::to_string(board.hash32) +
                     "\thash64=" + std::to_string(board.hash64) + "\tcore=" + board.core_state() +
                     "\tconsistent=" + (board.consistent() ? "true" : "false"));
        }
        else if (command == "perft")
        {
            schema(fields, {"depth"}, {"depth"});
            int depth = static_cast<int>(number(fields.at("depth"), 0, 3));
            auto &board = engine_.board();
            std::string before = board.state(), divide;
            std::uint64_t nodes = depth == 0 ? 1 : 0;
            if (depth > 0)
                for (auto move : board.legal_moves())
                {
                    auto undo = board.make_move(move);
                    auto count = board.perft(depth - 1);
                    board.unmake_move(undo);
                    if (!divide.empty())
                        divide += ',';
                    divide += xiangqi::format_move(move) + ':' + std::to_string(count);
                    nodes += count;
                }
            if (before != board.state() || !board.consistent())
                throw xiangqi::EngineError("INTERNAL_ERROR", "perft failed to restore board");
            emit(id, "ok", "nodes=" + std::to_string(nodes) + "\tdivide=" + divide + "\tstate_restored=true");
        }
        else if (command == "evaluate")
        {
            schema(fields, {"phase"});
            auto phase = fields.count("phase") ? fields.at("phase") : "current";
            auto board = engine_.board();
            if (phase != "current")
                xiangqi::prepare_evaluation(board, phase);
            emit(id, "ok", "report=" + xiangqi::evaluation_json(xiangqi::evaluate(board)));
        }
        else if (command == "eval-tables")
        {
            schema(fields, {});
            emit(id, "ok", "report=" + xiangqi::evaluation_tables_json());
        }
        else if (command == "search")
        {
            start(id, fields);
        }
        else if (command == "stop")
        {
            stop(id, fields);
        }
        else
            throw xiangqi::EngineError("UNKNOWN_COMMAND", "unknown command");
    }
    catch (const xiangqi::EngineError &error)
    {
        emit(id, "error", "code=" + error.code() + "\tmessage=" + error.what());
    }
    catch (const std::exception &)
    {
        emit(id, "error", "code=INTERNAL_ERROR\tmessage=unexpected engine exception");
    }
    return true;
}
} // namespace xiangqi::wire
