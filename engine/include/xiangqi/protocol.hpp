#pragma once
#include "xiangqi/board.hpp"
#include <map>
#include <memory>
#include <mutex>
#include <thread>
#include <set>

namespace xiangqi::wire
{
using Fields = std::map<std::string, std::string>;
inline constexpr std::size_t max_line = 65536;
bool token(const std::string &text);
std::vector<std::string> split(const std::string &input, char delimiter);
void schema(const Fields &, const std::set<std::string> &, const std::set<std::string> &required = {});
std::uint64_t number(const std::string &, std::uint64_t minimum, std::uint64_t maximum);
std::string list_moves(std::vector<Move> moves);
class Session
{
  public:
    ~Session();
    bool handle(const std::string &line);
    void emit(const std::string &id, const std::string &status, const std::string &fields = "");

  private:
    struct Job
    {
        std::atomic_bool cancelled{false}, done{false};
        std::string request_id, game_id, version, search_id;
        std::thread thread;
    };
    RulesEngine engine_;
    std::shared_ptr<Job> job_;
    std::mutex output_;
    void start(const std::string &id, const Fields &fields);
    void stop(const std::string &id, const Fields &fields);
    void join_finished();
    void shutdown();
};
} // namespace xiangqi::wire
