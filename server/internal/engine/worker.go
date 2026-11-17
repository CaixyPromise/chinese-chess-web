package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

func (w *Worker) write(frame string) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	_, e := io.WriteString(w.in, frame)
	return e
}
func parseFrame(line string) (string, string, map[string]string, error) {
	p := strings.Split(line, "\t")
	if len(p) < 3 || p[0] != "xq1" {
		return "", "", nil, errors.New("invalid native envelope")
	}
	fields := map[string]string{}
	for _, part := range p[3:] {
		k, v, ok := strings.Cut(part, "=")
		if !ok || k == "" {
			return "", "", nil, errors.New("malformed native field")
		}
		if _, exists := fields[k]; exists {
			return "", "", nil, errors.New("duplicate native field")
		}
		fields[k] = v
	}
	return p[1], p[2], fields, nil
}
func identity(command string) map[string]string {
	fields := map[string]string{}
	for _, part := range strings.Split(command, "\t")[1:] {
		k, v, ok := strings.Cut(part, "=")
		if ok {
			fields[k] = v
		}
	}
	return fields
}

// One lease reads an accepted task through its sole terminal result; info is diagnostic.
func (w *Worker) Call(ctx context.Context, command string) (map[string]string, error) {
	return w.CallWithInfo(ctx, command, nil)
}

// CallWithInfo observes complete iterations on the reading goroutine. The callback
// must not call this worker or retain/mutate its argument concurrently.
func (w *Worker) CallWithInfo(ctx context.Context, command string, onInfo func(map[string]string)) (map[string]string, error) {
	if strings.ContainsAny(command, "\r\n") || len(command) > 64000 {
		return nil, errors.New("invalid native frame")
	}
	w.id++
	id := strconv.FormatUint(w.id, 10)
	stopID := id + ".stop"
	search := strings.HasPrefix(command, "search\t")
	task := identity(command)
	type answer struct {
		fields map[string]string
		err    error
	}
	ready := make(chan answer, 1)
	go func() {
		if e := w.write("xq1\t" + id + "\t" + command + "\n"); e != nil {
			ready <- answer{err: e}
			return
		}
		accepted := false
		for frames := 0; frames < 128; frames++ {
			if !w.out.Scan() {
				ready <- answer{err: fmt.Errorf("native output ended: %v", w.out.Err())}
				return
			}
			frameID, status, fields, e := parseFrame(w.out.Text())
			if e != nil {
				ready <- answer{err: e}
				return
			}
			if search && frameID == stopID && status == "ok" {
				continue
			}
			if frameID != id {
				ready <- answer{err: errors.New("native frame identity mismatch")}
				return
			}
			if status == "error" {
				ready <- answer{err: &Error{Code: fields["code"]}}
				return
			}
			event := search && (status == "info" || status == "bestmove" || fields["accepted"] == "true")
			if event {
				for _, key := range []string{"game_id", "position_version", "search_id"} {
					if fields[key] != task[key] {
						ready <- answer{err: errors.New("native task identity mismatch")}
						return
					}
				}
			}
			if search && status == "ok" && fields["accepted"] == "true" {
				if accepted {
					ready <- answer{err: errors.New("duplicate native acceptance")}
					return
				}
				accepted = true
				continue
			}
			if status == "info" && accepted {
				if onInfo != nil {
					onInfo(fields)
				}
				continue
			}
			if status == "bestmove" && accepted || status == "ok" {
				ready <- answer{fields: fields}
				return
			}
			ready <- answer{err: errors.New("unexpected native frame")}
			return
		}
		ready <- answer{err: errors.New("too many native frames")}
	}()
	select {
	case result := <-ready:
		return result.fields, result.err
	case <-ctx.Done():
		if search && task["search_id"] != "" {
			written := make(chan struct{})
			go func() {
				_ = w.write("xq1\t" + stopID + "\tstop\tsearch_id=" + task["search_id"] + "\n")
				close(written)
			}()
			timer := time.NewTimer(200 * time.Millisecond)
			drained := false
			select {
			case <-ready:
				drained = true
			case <-timer.C:
			}
			timer.Stop()
			w.close()
			<-written
			if !drained {
				<-ready
			}
		} else {
			w.close()
			<-ready
		}
		return nil, ctx.Err()
	}
}
