package processing

// Kind is a Skill's pipeline stage. The integer ordering of these constants IS
// the documented execution precedence — a lower Kind runs before a higher one so
// that later stages consume earlier outputs:
//
//	download > convert > analyze > research > reply
//
// (processing-pipeline.md §5). OrderByPrecedence relies on this ordering, which
// matches skill_dispatch.Kind so the two modules agree on stage precedence.
type Kind int

const (
	// KindDownload fetches raw media/attachments (Boba/MeTube/Download Manager).
	KindDownload Kind = iota
	// KindConvert generates optimized "…-web" renditions while keeping the raw.
	KindConvert
	// KindAnalyze extracts meaning: Vision, OCR, transcript, classification.
	KindAnalyze
	// KindResearch runs multi-pass deep web research → docs → Skills growth.
	KindResearch
	// KindReply posts the status reply to the source thread and marks processed.
	KindReply
)

// String returns the canonical stage name.
func (k Kind) String() string {
	switch k {
	case KindDownload:
		return "download"
	case KindConvert:
		return "convert"
	case KindAnalyze:
		return "analyze"
	case KindResearch:
		return "research"
	case KindReply:
		return "reply"
	default:
		return "unknown"
	}
}

// Valid reports whether k is one of the defined stages.
func (k Kind) Valid() bool {
	return k >= KindDownload && k <= KindReply
}
