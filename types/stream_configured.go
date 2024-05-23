package types

import (
	"fmt"

	"github.com/piyushsingariya/shift/utils"
)

// Input/Processed object for Stream
type ConfiguredStream struct {
	Stream         *Stream      `json:"stream,omitempty"`
	SyncMode       SyncMode     `json:"sync_mode,omitempty"`       // Mode being used for syncing data
	CursorField    string       `json:"cursor_field,omitempty"`    // Column being used as cursor; MUST NOT BE mutated
	ExcludeColumns []string     `json:"exclude_columns,omitempty"` // TODO: Implement excluding columns from fetching
	CursorValue    any          `json:"-"`                         // Cached initial state value
	batchSize      int64        `json:"-"`                         // Batch size for syncing data
	state          *StreamState `json:"-"`                         // in-memory state copy for individual stream

	// DestinationSyncMode string   `json:"destination_sync_mode,omitempty"`
}

func (s *ConfiguredStream) ID() string {
	return s.Stream.ID()
}

func (s *ConfiguredStream) Self() *ConfiguredStream {
	return s
}

func (s *ConfiguredStream) Name() string {
	return s.Stream.Name
}

func (s *ConfiguredStream) GetStream() *Stream {
	return s.Stream
}

func (s *ConfiguredStream) Namespace() string {
	return s.Stream.Namespace
}

func (s *ConfiguredStream) Schema() *TypeSchema {
	return s.Stream.Schema
}

func (s *ConfiguredStream) SupportedSyncModes() *Set[SyncMode] {
	return s.Stream.SupportedSyncModes
}

func (s *ConfiguredStream) GetSyncMode() SyncMode {
	return s.SyncMode
}

func (s *ConfiguredStream) Cursor() string {
	return s.CursorField
}

func (s *ConfiguredStream) InitialState() any {
	return s.CursorValue
}

func (s *ConfiguredStream) SetState(value any) {
	if s.state == nil {
		s.state = &StreamState{
			Stream:    s.Name(),
			Namespace: s.Namespace(),
			State: map[string]any{
				s.Cursor(): value,
			},
		}
		return
	}

	s.state.State[s.Cursor()] = value
}

func (s *ConfiguredStream) GetState() any {
	if s.state == nil || s.state.State == nil {
		return nil
	}
	return s.state.State[s.Cursor()]
}

func (s *ConfiguredStream) BatchSize() int64 {
	return s.batchSize
}

func (s *ConfiguredStream) SetBatchSize(size int64) {
	s.batchSize = size
}

// Returns empty and missing
func (s *ConfiguredStream) SetupState(state *State) error {
	if !state.IsZero() {
		i, contains := utils.ArrayContains(state.Streams, func(elem *StreamState) bool {
			return elem.Namespace == s.Namespace() && elem.Stream == s.Name()
		})
		if contains {
			value, found := state.Streams[i].State[s.CursorField]
			if !found {
				return ErrStateCursorMissing
			}

			s.CursorValue = value

			return nil
		}

		return ErrStateMissing
	}

	return nil
}

// Validate Configured Stream with Source Stream
func (s *ConfiguredStream) Validate(source *Stream) error {
	if !utils.ExistInArray(source.SupportedSyncModes.Array(), s.SyncMode) {
		return fmt.Errorf("invalid sync mode[%s]; valid are %v", s.SyncMode, s.SupportedSyncModes().Array())
	}

	if !utils.ExistInArray(source.DefaultCursorFields.Array(), s.CursorField) {
		return fmt.Errorf("invalid cursor field [%s]; valid are %v", s.SyncMode, s.SupportedSyncModes())
	}

	if !source.SourceDefinedPrimaryKey.ProperSubsetOf(s.Stream.SourceDefinedPrimaryKey) {
		return fmt.Errorf("differnce found with primary keys: %v", source.SourceDefinedPrimaryKey.Difference(s.Stream.SourceDefinedPrimaryKey).Array())
	}

	return nil
}