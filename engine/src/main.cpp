#include "xiangqi/protocol.hpp"
#include <iostream>

int main()
{
    xiangqi::wire::Session session;
    std::string line;
    bool overflow = false;
    char c;
    while (std::cin.get(c))
    {
        if (c == '\n')
        {
            if (overflow)
                session.emit("-", "error", "code=LINE_TOO_LONG\tmessage=max request is 65536 bytes");
            else if (!session.handle(line))
                return 0;
            line.clear();
            overflow = false;
        }
        else if (line.size() < xiangqi::wire::max_line)
            line += c;
        else
            overflow = true;
    }
    // A truncated final frame must never be executed.
    if (overflow || !line.empty())
    {
        session.emit("-", "error", "code=TRUNCATED_FRAME\tmessage=request must end in LF");
        return 1;
    }
    return 0;
}
