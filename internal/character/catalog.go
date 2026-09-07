package character

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"streaming-agent/internal/live"
)

type Store interface {
	PublishCharacter(context.Context, live.CharacterProfile) error
	Character(context.Context, string) (live.CharacterProfile, error)
	Characters(context.Context) ([]live.CharacterProfile, error)
	SelectCharacter(context.Context, string) error
}
type Catalog struct {
	Store Store
	Root  string
}

func (c Catalog) Import(data []byte) (live.ReferenceAsset, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return live.ReferenceAsset{}, fmt.Errorf("decode reference: %w", err)
	}
	if cfg.Width < 256 || cfg.Height < 256 || len(data) > 10<<20 {
		return live.ReferenceAsset{}, errors.New("reference must be at least 256px and at most 10MB")
	}
	id := fmt.Sprintf("%x", sha256.Sum256(data))
	rel := filepath.Join("media", id+"."+format)
	path := filepath.Join(c.Root, rel)
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return live.ReferenceAsset{}, err
	}
	if err = os.WriteFile(path, data, 0o640); err != nil {
		return live.ReferenceAsset{}, err
	}
	return live.ReferenceAsset{ID: id, Path: rel, MIME: "image/" + format, Width: cfg.Width, Height: cfg.Height}, nil
}
func (c Catalog) Publish(ctx context.Context, p live.CharacterProfile, first, last []byte) (live.CharacterProfile, error) {
	if p.CharacterID == "" || p.Name == "" || p.Description == "" {
		return p, errors.New("character id, name and description required")
	}
	var err error
	if len(first) == 0 || len(last) == 0 {
		old, _, resolveErr := c.Resolve(ctx, p.ID)
		if resolveErr != nil {
			return p, resolveErr
		}
		if len(first) == 0 {
			first, err = os.ReadFile(filepath.Join(c.Root, old.FirstFrame.Path))
			if err != nil {
				return p, err
			}
		}
		if len(last) == 0 {
			last, err = os.ReadFile(filepath.Join(c.Root, old.LastFrame.Path))
			if err != nil {
				return p, err
			}
		}
	}
	if p.FirstFrame, err = c.Import(first); err != nil {
		return p, err
	}
	if p.LastFrame, err = c.Import(last); err != nil {
		return p, err
	}
	p.ID = ""
	data, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	p.ID = fmt.Sprintf("%x", sha256.Sum256(data))
	return p, c.Store.PublishCharacter(ctx, p)
}
func (c Catalog) Resolve(ctx context.Context, id string) (live.CharacterProfile, map[string]string, error) {
	p, err := c.Store.Character(ctx, id)
	if err != nil {
		return p, nil, err
	}
	refs := map[string]live.ReferenceAsset{"chat-live-start": p.FirstFrame, "chat-live-end": p.LastFrame}
	anchors := make(map[string]string, len(refs))
	for key, ref := range refs {
		if !filepath.IsLocal(ref.Path) {
			return p, nil, errors.New("invalid media path")
		}
		data, err := os.ReadFile(filepath.Join(c.Root, ref.Path))
		if err != nil {
			return p, nil, fmt.Errorf("load reference %s: %w", ref.ID, err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != ref.ID {
			return p, nil, errors.New("reference checksum mismatch")
		}
		anchors[key] = "data:" + ref.MIME + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return p, anchors, nil
}
