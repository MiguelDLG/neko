package draw

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/m1k1o/neko/server/pkg/auth"
	"github.com/m1k1o/neko/server/pkg/types"
	"github.com/m1k1o/neko/server/pkg/utils"
)

var (
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	idRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

const (
	minWidth = 0.0005
	maxWidth = 0.05
)

func NewManager(
	sessions types.SessionManager,
	config *Config,
) *Manager {
	logger := log.With().Str("module", "draw").Logger()

	return &Manager{
		logger:   logger,
		config:   config,
		sessions: sessions,
		board:    newBoard(config.MaxStrokes, config.MaxPoints),
	}
}

type Manager struct {
	logger   zerolog.Logger
	config   *Config
	sessions types.SessionManager
	board    *board
}

// Settings can be set globally (session settings plugins map) and per user
// (member profile plugins map), both under the "draw." prefix:
//
//	draw.can_draw: false
type Settings struct {
	CanDraw bool `json:"can_draw" mapstructure:"can_draw"`
}

func (m *Manager) settingsForSession(session types.Session) (Settings, error) {
	settings := Settings{
		CanDraw: true, // defaults to true
	}
	err := m.sessions.Settings().Plugins.Unmarshal(PluginName, &settings)
	if err != nil && !errors.Is(err, types.ErrPluginSettingsNotFound) {
		return Settings{}, fmt.Errorf("unable to unmarshal %s plugin settings from global settings: %w", PluginName, err)
	}

	profile := Settings{
		CanDraw: true, // defaults to true
	}
	err = session.Profile().Plugins.Unmarshal(PluginName, &profile)
	if err != nil && !errors.Is(err, types.ErrPluginSettingsNotFound) {
		return Settings{}, fmt.Errorf("unable to unmarshal %s plugin settings from profile: %w", PluginName, err)
	}

	return Settings{
		CanDraw: m.config.Enabled && (settings.CanDraw || session.Profile().IsAdmin) && profile.CanDraw,
	}, nil
}

func (m *Manager) canDraw(session types.Session) bool {
	settings, err := m.settingsForSession(session)
	if err != nil {
		m.logger.Error().Err(err).Msg("error checking draw permissions for this session")
		return false
	}
	return settings.CanDraw
}

func (m *Manager) canClear(session types.Session) bool {
	if !m.canDraw(session) {
		return false
	}
	if m.config.ClearAdminOnly && !session.Profile().IsAdmin {
		return false
	}
	return true
}

// broadcast sends a message to every connected session, optionally skipping
// the author (who already rendered its own stroke locally).
func (m *Manager) broadcast(event string, payload any, skip types.Session) {
	m.sessions.Range(func(s types.Session) bool {
		if skip != nil && s.ID() == skip.ID() {
			return true
		}
		if s.State().IsConnected {
			s.Send(event, payload)
		}
		return true
	})
}

func (m *Manager) validateStroke(p *StrokePayload) error {
	if !idRe.MatchString(p.ID) {
		return errors.New("invalid stroke id")
	}
	if !colorRe.MatchString(p.Color) {
		return errors.New("invalid stroke color")
	}
	if math.IsNaN(p.Width) || p.Width < minWidth || p.Width > maxWidth {
		return errors.New("invalid stroke width")
	}
	if len(p.Points) == 0 {
		return errors.New("stroke has no points")
	}
	if len(p.Points) > m.config.MaxPoints {
		return errors.New("too many points in stroke")
	}
	for _, pt := range p.Points {
		for _, v := range pt {
			if math.IsNaN(v) || v < -0.05 || v > 1.05 {
				return errors.New("point out of bounds")
			}
		}
	}
	return nil
}

func (m *Manager) handleStroke(session types.Session, payload json.RawMessage) {
	var p StrokePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		m.logger.Error().Err(err).Msg("failed to unmarshal draw stroke")
		return
	}

	if !m.canDraw(session) {
		m.logger.Warn().Str("session_id", session.ID()).Msg("not allowed to draw")
		return
	}

	if err := m.validateStroke(&p); err != nil {
		m.logger.Warn().Err(err).Str("session_id", session.ID()).Msg("rejected draw stroke")
		return
	}

	// fading strokes are relayed only, never persisted (laser pointer style)
	if !p.Fade {
		if !m.board.upsert(session.ID(), p) {
			m.logger.Warn().Str("session_id", session.ID()).Str("stroke_id", p.ID).Msg("rejected draw stroke upsert")
			return
		}
	}

	m.broadcast(DRAW_STROKE, StrokeMessage{
		StrokePayload: p,
		UserID:        session.ID(),
	}, session)
}

func (m *Manager) handleUndo(session types.Session, payload json.RawMessage) {
	var p UndoPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			m.logger.Error().Err(err).Msg("failed to unmarshal draw undo")
			return
		}
	}

	if !m.canDraw(session) {
		m.logger.Warn().Str("session_id", session.ID()).Msg("not allowed to draw")
		return
	}

	if p.ID != "" && !idRe.MatchString(p.ID) {
		m.logger.Warn().Str("session_id", session.ID()).Msg("rejected draw undo: invalid id")
		return
	}

	removed := m.board.remove(session.ID(), p.ID, session.Profile().IsAdmin)
	if removed == "" {
		return
	}

	m.broadcast(DRAW_UNDO, UndoMessage{
		ID:     removed,
		UserID: session.ID(),
	}, nil)
}

func (m *Manager) handleClear(session types.Session) {
	if !m.canClear(session) {
		m.logger.Warn().Str("session_id", session.ID()).Msg("not allowed to clear the drawing board")
		return
	}

	m.board.clear()

	m.broadcast(DRAW_CLEAR, ClearMessage{
		UserID: session.ID(),
	}, nil)
}

func (m *Manager) Start() error {
	// send the current board once a user connects
	m.sessions.OnConnected(func(session types.Session) {
		session.Send(DRAW_INIT, Init{
			Enabled: m.config.Enabled,
			CanDraw: m.canDraw(session),
			Strokes: m.board.snapshot(),
		})
	})

	return nil
}

func (m *Manager) Shutdown() error {
	return nil
}

func (m *Manager) Route(r types.Router) {
	r.Get("/", m.getBoardHandler)
	r.With(auth.AdminsOnly).Delete("/", m.clearHandler)
}

func (m *Manager) WebSocketHandler(session types.Session, msg types.WebSocketMessage) bool {
	switch msg.Event {
	case DRAW_STROKE:
		m.handleStroke(session, msg.Payload)
		return true
	case DRAW_UNDO:
		m.handleUndo(session, msg.Payload)
		return true
	case DRAW_CLEAR:
		m.handleClear(session)
		return true
	}
	return false
}

func (m *Manager) getBoardHandler(w http.ResponseWriter, r *http.Request) error {
	if _, ok := auth.GetSession(r); !ok {
		return utils.HttpUnauthorized("session not found")
	}

	return utils.HttpSuccess(w, Init{
		Enabled: m.config.Enabled,
		CanDraw: false,
		Strokes: m.board.snapshot(),
	})
}

func (m *Manager) clearHandler(w http.ResponseWriter, r *http.Request) error {
	session, ok := auth.GetSession(r)
	if !ok {
		return utils.HttpUnauthorized("session not found")
	}

	m.board.clear()

	m.broadcast(DRAW_CLEAR, ClearMessage{
		UserID: session.ID(),
	}, nil)

	return utils.HttpSuccess(w)
}
