package clipboard

type ClipboardFormat string

type ClipboardTextData struct {
	Value    string `json:"value"`
	Encoding string `json:"encoding,omitempty"`
	Length   int    `json:"length,omitempty"`
}

type ClipboardImageData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type ClipboardFileEntry struct {
	Name     string `json:"name"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Path     string `json:"path,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

type ClipboardContent struct {
	Format   ClipboardFormat      `json:"format"`
	Text     *ClipboardTextData   `json:"text,omitempty"`
	Image    *ClipboardImageData  `json:"image,omitempty"`
	Files    []ClipboardFileEntry `json:"files,omitempty"`
	Metadata map[string]string    `json:"metadata,omitempty"`
}

type ClipboardSnapshot struct {
	Sequence   uint64            `json:"sequence"`
	CapturedAt string            `json:"capturedAt"`
	Source     string            `json:"source,omitempty"`
	Content    *ClipboardContent `json:"content,omitempty"`
}

type ClipboardStateEnvelope struct {
	RequestID string            `json:"requestId,omitempty"`
	Snapshot  ClipboardSnapshot `json:"snapshot"`
}

type ClipboardTriggerCondition struct {
	Formats       []ClipboardFormat `json:"formats,omitempty"`
	Pattern       string            `json:"pattern,omitempty"`
	CaseSensitive bool              `json:"caseSensitive,omitempty"`
}

type ClipboardTriggerAction struct {
	Type          string         `json:"type"`
	Configuration map[string]any `json:"configuration,omitempty"`
}

type ClipboardTrigger struct {
	ID          string                    `json:"id"`
	Label       string                    `json:"label"`
	Description string                    `json:"description,omitempty"`
	Condition   ClipboardTriggerCondition `json:"condition"`
	Action      ClipboardTriggerAction    `json:"action"`
	CreatedAt   string                    `json:"createdAt"`
	UpdatedAt   string                    `json:"updatedAt,omitempty"`
	Active      bool                      `json:"active"`
}

type ClipboardTriggerMatch struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type ClipboardTriggerEvent struct {
	EventID      string                  `json:"eventId"`
	TriggerID    string                  `json:"triggerId"`
	TriggerLabel string                  `json:"triggerLabel"`
	CapturedAt   string                  `json:"capturedAt"`
	Sequence     uint64                  `json:"sequence"`
	RequestID    string                  `json:"requestId,omitempty"`
	Matches      []ClipboardTriggerMatch `json:"matches,omitempty"`
	Content      ClipboardContent        `json:"content"`
	Action       ClipboardTriggerAction  `json:"action"`
}

type ClipboardCommandPayload struct {
	Action    string             `json:"action"`
	RequestID string             `json:"requestId,omitempty"`
	Content   *ClipboardContent  `json:"content,omitempty"`
	Triggers  []ClipboardTrigger `json:"triggers,omitempty"`
	Source    string             `json:"source,omitempty"`
	Sequence  *uint64            `json:"sequence,omitempty"`
}

type ClipboardEventEnvelope struct {
	Events []ClipboardTriggerEvent `json:"events"`
}
