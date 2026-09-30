// Package geofences trata as cercas circulares e a avaliação de entrada/saída.
package geofences

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

type Geofence struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`

	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMeters float64 `json:"radiusMeters"`

	Active bool `json:"active"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Input struct {
	Name         string  `json:"name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMeters float64 `json:"radiusMeters"`
	Active       *bool   `json:"active"`
}

const columns = `id, name, latitude, longitude, radius_meters, active, created_at, updated_at`

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db: db} }

func scan(row database.Scanner) (*Geofence, error) {
	var g Geofence
	err := row.Scan(&g.ID, &g.Name, &g.Latitude, &g.Longitude, &g.RadiusMeters,
		&g.Active, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &g, nil
}

func (r *Repository) Create(ctx context.Context, in Input) (*Geofence, error) {
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	return scan(r.db.QueryRow(ctx, `
		INSERT INTO geofences (name, latitude, longitude, radius_meters, active)
		VALUES ($1, $2, $3, $4, $5) RETURNING `+columns,
		in.Name, in.Latitude, in.Longitude, in.RadiusMeters, active))
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, in Input) (*Geofence, error) {
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	return scan(r.db.QueryRow(ctx, `
		UPDATE geofences SET name = $2, latitude = $3, longitude = $4,
			radius_meters = $5, active = $6, updated_at = NOW()
		WHERE id = $1 RETURNING `+columns,
		id, in.Name, in.Latitude, in.Longitude, in.RadiusMeters, active))
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM geofences WHERE id = $1`, id)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context) ([]*Geofence, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+` FROM geofences ORDER BY name`)
	if err != nil {
		return nil, database.MapError(err)
	}
	defer rows.Close()

	out := []*Geofence{}
	for rows.Next() {
		g, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Service mantém as cercas ativas em memória: a avaliação roda a cada posição
// recebida e não pode depender de uma consulta ao banco.
type Service struct {
	repo *Repository

	mu     sync.RWMutex
	cached []*Geofence
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// Refresh recarrega o cache a partir do banco.
func (s *Service) Refresh(ctx context.Context) error {
	all, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	active := make([]*Geofence, 0, len(all))
	for _, g := range all {
		if g.Active {
			active = append(active, g)
		}
	}
	s.mu.Lock()
	s.cached = active
	s.mu.Unlock()
	return nil
}

func (s *Service) List(ctx context.Context) ([]*Geofence, error) { return s.repo.List(ctx) }

func (s *Service) Create(ctx context.Context, in Input) (*Geofence, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	g, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	_ = s.Refresh(ctx)
	return g, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (*Geofence, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	g, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, err
	}
	_ = s.Refresh(ctx)
	return g, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.Refresh(ctx)
}

// Inside devolve os IDs das cercas que contêm o ponto.
func (s *Service) Inside(lat, lon float64) []uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []uuid.UUID
	for _, g := range s.cached {
		if DistanceMeters(lat, lon, g.Latitude, g.Longitude) <= g.RadiusMeters {
			out = append(out, g.ID)
		}
	}
	return out
}

// Name devolve o nome da cerca a partir do cache.
func (s *Service) Name(id uuid.UUID) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.cached {
		if g.ID == id {
			return g.Name
		}
	}
	return ""
}

// ValidationError descreve um payload recusado.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" {
		return ValidationError{Message: "o nome da cerca é obrigatório"}
	}
	if in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180 {
		return ValidationError{Message: "coordenadas da cerca fora de faixa"}
	}
	if in.RadiusMeters < 20 || in.RadiusMeters > 200000 {
		return ValidationError{Message: "raio fora da faixa 20..200000 metros"}
	}
	return nil
}

const earthRadiusMeters = 6371000.0

// DistanceMeters devolve a distância entre dois pontos pela fórmula de haversine.
func DistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	dPhi := (lat2 - lat1) * math.Pi / 180
	dLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return 2 * earthRadiusMeters * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
