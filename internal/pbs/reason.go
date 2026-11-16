package pbs

import (
	"regexp"
	"strings"
)

// Wait reasons for jobs that are not running. PBS writes why a job is waiting
// into the job's `comment` (set by the scheduler) and records holds in
// `Hold_Types`; WaitReason turns those into a small, fixed set of reasons
// plus a detail (the queue, resource or hold type involved).
const (
	ReasonUserLimit     = "user_limit"            // user reached a per-user run/resource limit
	ReasonGroupLimit    = "group_limit"           // group reached a limit
	ReasonProjectLimit  = "project_limit"         // project reached a limit
	ReasonQueueLimit    = "queue_limit"           // queue-wide limit reached
	ReasonServerLimit   = "server_limit"          // server-wide limit reached
	ReasonResource      = "insufficient_resource" // not enough free ngpus/ncpus/mem/nodes right now
	ReasonCanNeverRun   = "can_never_run"         // request can never be satisfied (wrong host, model, size)
	ReasonTopJob        = "top_job"               // blocked so a higher-priority (top) job can start
	ReasonBudget        = "budget"                // budget/allocation hook refused the job
	ReasonQueueStopped  = "queue_stopped"         // queue not started / scheduling off
	ReasonDedicatedTime = "dedicated_time"        // dedicated or prime time restriction
	ReasonPBSError      = "pbs_error"             // other PBS/hook error
	ReasonHeldUser      = "held_user"             // qhold by the owner (Hold_Types u)
	ReasonHeldOperator  = "held_operator"         // held by an operator/admin (Hold_Types o)
	ReasonHeldSystem    = "held_system"           // system hold (Hold_Types s)
	ReasonHeldFailed    = "held_failed_runs"      // held after too many failed attempts to start
	ReasonHeldPassword  = "held_bad_password"     // Hold_Types p
	ReasonStartTime     = "start_time"            // state W: waiting for its requested start time (-a)
	ReasonArraySubjob   = "array_subjob"          // queued subjob of an array that has started
	ReasonNotEvaluated  = "not_evaluated"         // no comment yet: the scheduler has not looked at it
	ReasonDependency    = "dependency"            // waiting on another job (-W depend)
	ReasonOther         = "other"                 // a comment we don't recognise (detail has the text)
)

var (
	// "User has reached queue small running job limit." / "Group has reached server running limit."
	reachedLimitRe = regexp.MustCompile(`(?i)\b(user|group|project)(?: \S+)? has reached (.+?) limit`)
	// "would exceed user bob's limit on resource ngpus in queue gpu"
	// "would exceed per-user limit on resource ngpus in complex"
	// "would exceed overall limit on resource ngpus in queue gpu"
	exceedLimitRe = regexp.MustCompile(`(?i)would exceed (?:(user|group|project) \S+'s|per-(user|group|project)|overall) limit on resource (\S+) in (complex|queue (\S+))`)
	// "Queue small job limit has been reached." / "Server job limit has been reached"
	queueServerLimitRe = regexp.MustCompile(`(?i)\b(queue (\S+)|server) .*limit`)
	// "Insufficient amount of resource: ngpus (R: 1 A: 0 T: 168)", also "server resource" / "queue resource"
	resourceRe = regexp.MustCompile(`(?i)insufficient amount of (?:server |queue )?resource: ?(\w+)`)
	digitsRe   = regexp.MustCompile(`\d+`)
	spaceRe    = regexp.MustCompile(`\s+`)
)

// WaitReason classifies why a job that is not running is waiting.
// state is job_state, comment the job comment and holdTypes the Hold_Types value.
func WaitReason(state, comment, holdTypes string) (reason, detail string) {
	comment = strings.TrimSpace(comment)
	lower := strings.ToLower(comment)

	if strings.Contains(lower, "too many failed attempts") {
		return ReasonHeldFailed, ""
	}

	if state == "H" {
		switch {
		case strings.Contains(holdTypes, "s"):
			return ReasonHeldSystem, holdTypes
		case strings.Contains(holdTypes, "o"):
			return ReasonHeldOperator, holdTypes
		case strings.Contains(holdTypes, "p"):
			return ReasonHeldPassword, holdTypes
		case strings.Contains(holdTypes, "u"):
			return ReasonHeldUser, holdTypes
		}
		if strings.Contains(lower, "depend") {
			return ReasonDependency, ""
		}
		return ReasonHeldSystem, holdTypes
	}

	if state == "W" {
		return ReasonStartTime, ""
	}

	if comment == "" {
		return ReasonNotEvaluated, ""
	}

	canNever := strings.HasPrefix(lower, "can never run")

	switch {
	case strings.Contains(lower, "budget"):
		return ReasonBudget, budgetDetail(comment)
	case canNever:
		if m := resourceRe.FindStringSubmatch(comment); m != nil {
			return ReasonCanNeverRun, m[1]
		}
		return ReasonCanNeverRun, shortText(strings.TrimLeft(comment[len("can never run"):], ": "))
	}

	if m := reachedLimitRe.FindStringSubmatch(comment); m != nil {
		return strings.ToLower(m[1]) + "_limit", strings.TrimSpace(m[2])
	}
	if m := exceedLimitRe.FindStringSubmatch(comment); m != nil {
		who := strings.ToLower(m[1] + m[2])
		scope := "server"
		if m[5] != "" {
			scope = "queue " + m[5]
		}
		if who == "" { // "overall" limit
			if m[5] != "" {
				return ReasonQueueLimit, scope + " " + m[3]
			}
			return ReasonServerLimit, scope + " " + m[3]
		}
		return who + "_limit", scope + " " + m[3]
	}

	switch {
	case resourceRe.MatchString(comment):
		return ReasonResource, resourceRe.FindStringSubmatch(comment)[1]
	case strings.Contains(lower, "not enough free nodes"), strings.Contains(lower, "no available resources on nodes"):
		return ReasonResource, "nodes"
	case strings.Contains(lower, "exclusive"):
		return ReasonResource, "exclusive_node"
	case strings.Contains(lower, "top job"), strings.Contains(lower, "drain"):
		return ReasonTopJob, ""
	case strings.Contains(lower, "queue") && (strings.Contains(lower, "not started") || strings.Contains(lower, "stopped")),
		strings.Contains(lower, "scheduling is not enabled"):
		return ReasonQueueStopped, ""
	case strings.Contains(lower, "dedicated time"), strings.Contains(lower, "prime"):
		return ReasonDedicatedTime, ""
	case strings.Contains(lower, "depend"):
		return ReasonDependency, ""
	case strings.HasPrefix(lower, "job array began"), strings.HasPrefix(lower, "job array"):
		return ReasonArraySubjob, ""
	}
	if m := queueServerLimitRe.FindStringSubmatch(comment); m != nil {
		if m[2] != "" {
			return ReasonQueueLimit, "queue " + m[2]
		}
		return ReasonServerLimit, "server"
	}
	if i := strings.Index(lower, "pbs error:"); i >= 0 {
		return ReasonPBSError, shortText(comment[i+len("pbs error:"):])
	}
	return ReasonOther, shortText(strings.TrimPrefix(comment, "Not Running:"))
}

// budgetDetail returns the hook's message without the hook name,
// e.g. "Project has already ended"
func budgetDetail(comment string) string {
	if i := strings.LastIndex(comment, ":"); i >= 0 && i < len(comment)-1 {
		comment = comment[i+1:]
	}
	return shortText(comment)
}

// shortText makes free text safe as a low-cardinality label: digits become N,
// whitespace is collapsed, trailing dots dropped, and it is cut to 60 characters
func shortText(s string) string {
	s = digitsRe.ReplaceAllString(s, "N")
	s = strings.Trim(spaceRe.ReplaceAllString(s, " "), " .")
	r := []rune(strings.ToValidUTF8(s, ""))
	if len(r) > 60 {
		r = r[:60]
	}
	return string(r)
}
