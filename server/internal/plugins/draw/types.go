package draw

import "time"

const PluginName = "draw"

const (
	// server -> client: sent once on connect, carries the current board
	DRAW_INIT = "draw/init"
	// client -> server -> clients: a stroke (or a continuation of a stroke
	// identified by the same id, allowing live drawing)
	DRAW_STROKE = "draw/stroke"
	// client -> server -> clients: remove one stroke by id
	DRAW_UNDO = "draw/undo"
	// client -> server -> clients: remove all strokes
	DRAW_CLEAR = "draw/clear"
)

// Point is a coordinate normalized to the video frame: 0..1 on both axes.
type Point [2]float64

// Stroke is a polyline drawn by one user. Width is a fraction of the video
// width so it scales uniformly on every viewer.
type Stroke struct {
	ID     string    `json:"id"`
	UserID string    `json:"user_id"`
	Color  string    `json:"color"`
	Width  float64   `json:"width"`
	Fade   bool      `json:"fade,omitempty"`
	Points []Point   `json:"points"`
	Time   time.Time `json:"time"`
}

// StrokePayload is what a client sends. The same id may be sent repeatedly
// with additional points while the stroke is still being drawn.
type StrokePayload struct {
	ID     string  `json:"id"`
	Color  string  `json:"color"`
	Width  float64 `json:"width"`
	Fade   bool    `json:"fade,omitempty"`
	Points []Point `json:"points"`
}

// StrokeMessage is the broadcast form: the client payload plus the author.
type StrokeMessage struct {
	StrokePayload
	UserID string `json:"user_id"`
}

type UndoPayload struct {
	ID string `json:"id,omitempty"`
}

type UndoMessage struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
}

type ClearMessage struct {
	UserID string `json:"user_id"`
}

type Init struct {
	Enabled bool     `json:"enabled"`
	CanDraw bool     `json:"can_draw"`
	Strokes []Stroke `json:"strokes"`
}
