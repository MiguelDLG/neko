package draw

import (
	"sync"
	"time"
)

// board is the server-side canonical list of strokes. It is the source of
// truth for late joiners and for undo/clear, so every mutation goes through
// it under a lock.
type board struct {
	mu         sync.RWMutex
	strokes    []*Stroke
	byID       map[string]*Stroke
	maxStrokes int
	maxPoints  int
}

func newBoard(maxStrokes, maxPoints int) *board {
	return &board{
		strokes:    make([]*Stroke, 0),
		byID:       make(map[string]*Stroke),
		maxStrokes: maxStrokes,
		maxPoints:  maxPoints,
	}
}

// upsert appends points to an existing stroke owned by userID, or creates a
// new stroke. It returns false when the id belongs to another user or the
// stroke is already full.
func (b *board) upsert(userID string, p StrokePayload) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if s, ok := b.byID[p.ID]; ok {
		if s.UserID != userID {
			return false
		}
		if len(s.Points)+len(p.Points) > b.maxPoints {
			return false
		}
		s.Points = append(s.Points, p.Points...)
		return true
	}

	if len(p.Points) > b.maxPoints {
		return false
	}

	s := &Stroke{
		ID:     p.ID,
		UserID: userID,
		Color:  p.Color,
		Width:  p.Width,
		Fade:   p.Fade,
		Points: append([]Point(nil), p.Points...),
		Time:   time.Now(),
	}
	b.strokes = append(b.strokes, s)
	b.byID[s.ID] = s

	// drop oldest strokes beyond the cap
	for len(b.strokes) > b.maxStrokes {
		old := b.strokes[0]
		b.strokes = b.strokes[1:]
		delete(b.byID, old.ID)
	}

	return true
}

// remove deletes the stroke with the given id. When id is empty the last
// stroke owned by userID is removed. When any is true, ownership is not
// checked (admins). It returns the removed stroke id, or "" when nothing
// was removed.
func (b *board) remove(userID string, id string, any bool) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if id == "" {
		for i := len(b.strokes) - 1; i >= 0; i-- {
			if b.strokes[i].UserID == userID {
				id = b.strokes[i].ID
				break
			}
		}
		if id == "" {
			return ""
		}
	}

	s, ok := b.byID[id]
	if !ok {
		return ""
	}
	if !any && s.UserID != userID {
		return ""
	}

	delete(b.byID, id)
	for i, cur := range b.strokes {
		if cur.ID == id {
			b.strokes = append(b.strokes[:i], b.strokes[i+1:]...)
			break
		}
	}
	return id
}

func (b *board) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.strokes = make([]*Stroke, 0)
	b.byID = make(map[string]*Stroke)
}

// snapshot returns a copy of all strokes for late joiners.
func (b *board) snapshot() []Stroke {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]Stroke, 0, len(b.strokes))
	for _, s := range b.strokes {
		cp := *s
		cp.Points = append([]Point(nil), s.Points...)
		out = append(out, cp)
	}
	return out
}
