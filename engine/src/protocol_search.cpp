#include "xiangqi/protocol.hpp"
#include <iostream>
#include <limits>
namespace xiangqi::wire
{
namespace
{
std::string identity(const std::string &game, const std::string &version, const std::string &search)
{
    return "game_id=" + game + "\tposition_version=" + version + "\tsearch_id=" + search;
}
std::string statistics(const SearchInfo &info)
{
    std::string pv;
    for (auto move : info.pv)
    {
        if (!pv.empty())
            pv += ',';
        pv += format_move(move);
    }
    return "score=" + std::to_string(info.score) + "\tdepth=" + std::to_string(info.completed_depth) +
           "\tcompleted_ns=" + std::to_string(info.completed_ns) + "\tnodes=" + std::to_string(info.nodes) +
           "\tevaluations=" + std::to_string(info.evaluations) + "\tcache_hits=" + std::to_string(info.cache_hits) +
           "\tcutoffs=" + std::to_string(info.cutoffs) + "\telapsed_ms=" + std::to_string(info.elapsed_ms) +
           "\tpv=" + pv;
}
} // namespace
void Session::emit(const std::string &id, const std::string &status, const std::string &fields)
{
    std::lock_guard<std::mutex> lock(output_);
    std::cout << "xq1\t" << id << '\t' << status;
    if (!fields.empty())
        std::cout << '\t' << fields;
    std::cout << '\n' << std::flush;
}
void Session::join_finished()
{
    if (job_ && job_->done.load(std::memory_order_acquire) && job_->thread.joinable())
        job_->thread.join();
}
void Session::shutdown()
{
    if (job_)
    {
        job_->cancelled.store(true, std::memory_order_relaxed);
        if (job_->thread.joinable())
            job_->thread.join();
    }
}
Session::~Session()
{
    shutdown();
}
void Session::start(const std::string &id, const Fields &f)
{
    schema(f, {"game_id", "position_version", "search_id", "depth", "time_ms", "nodes", "hash_mib", "root_move"},
           {"game_id", "position_version", "search_id", "depth", "time_ms", "hash_mib"});
    if (!token(f.at("game_id")) || !token(f.at("search_id")))
        throw EngineError("INVALID_ARGUMENT", "invalid task identity");
    number(f.at("position_version"), 0, std::numeric_limits<std::uint64_t>::max());
    SearchLimits limits;
    limits.max_depth = static_cast<std::uint32_t>(number(f.at("depth"), 1, 32));
    limits.time_ms = number(f.at("time_ms"), 1, 3600000);
    limits.hash_mib = static_cast<std::uint32_t>(number(f.at("hash_mib"), 1, 1024));
    if (f.count("nodes"))
        limits.max_nodes = number(f.at("nodes"), 0, std::numeric_limits<std::uint64_t>::max());
    engine_.board();
    if (f.count("root_move"))
    {
        try
        {
            limits.rootMove = parse_move(f.at("root_move"));
        }
        catch (const EngineError &)
        {
            throw EngineError("ILLEGAL_MOVE", "root_move is not a legal root move");
        }
        bool found = false;
        for (auto move : engine_.legal_moves())
            if (move.from == limits.rootMove->from && move.to == limits.rootMove->to)
            {
                found = true;
                break;
            }
        if (!found)
            throw EngineError("ILLEGAL_MOVE", "root_move is not a legal root move");
    }
    auto copy = engine_;
    auto job = std::make_shared<Job>();
    job->request_id = id;
    job->game_id = f.at("game_id");
    job->version = f.at("position_version");
    job->search_id = f.at("search_id");
    job_ = job;
    auto ids = identity(job->game_id, job->version, job->search_id);
    emit(id, "ok", "accepted=true\t" + ids);
    try
    {
        job->thread = std::thread(
            [this, job, copy = std::move(copy), limits, ids]() mutable
            {
                try
                {
                    auto result = copy.search(limits, job->cancelled, [&](const SearchInfo &info)
                                              { emit(job->request_id, "info", ids + '\t' + statistics(info)); });
                    std::string reason = result.reason == StopReason::deadline    ? "deadline"
                                         : result.reason == StopReason::cancelled ? "cancelled"
                                         : result.reason == StopReason::terminal  ? "terminal"
                                                                                  : "completed";
                    job->done.store(true, std::memory_order_release);
                    emit(job->request_id, "bestmove",
                         ids + "\tmove=" + (result.has_move ? format_move(result.best_move) : "none") +
                             "\tstop_reason=" + reason + "\tscore_reliable=" +
                             (result.info.completed_depth || result.reason == StopReason::terminal ? "true" : "false") +
                             '\t' + statistics(result.info));
                }
                catch (const EngineError &e)
                {
                    job->done.store(true, std::memory_order_release);
                    emit(job->request_id, "error", ids + "\tcode=" + e.code() + "\tmessage=" + e.what());
                }
                catch (const std::exception &)
                {
                    job->done.store(true, std::memory_order_release);
                    emit(job->request_id, "error", ids + "\tcode=INTERNAL_ERROR\tmessage=search failed");
                }
            });
    }
    catch (...)
    {
        job->done.store(true, std::memory_order_release);
        throw;
    }
}
void Session::stop(const std::string &id, const Fields &fields)
{
    schema(fields, {"search_id"}, {"search_id"});
    if (!token(fields.at("search_id")))
        throw EngineError("INVALID_ARGUMENT", "invalid search identity");
    if (!job_ || job_->search_id != fields.at("search_id"))
        throw EngineError("NO_ACTIVE_SEARCH", "no matching search");
    bool finished = job_->done.load(std::memory_order_acquire);
    if (!finished)
        job_->cancelled.store(true, std::memory_order_relaxed);
    emit(id, "ok",
         "search_id=" + job_->search_id + "\tstop_requested=" + (finished ? "false" : "true") +
             "\tfinished=" + (finished ? "true" : "false"));
}
} // namespace xiangqi::wire
