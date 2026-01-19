package model

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
)

// UUID scalar for GraphQL
type UUID uuid.UUID

func (u UUID) String() string {
	return uuid.UUID(u).String()
}

func (u UUID) UUID() uuid.UUID {
	return uuid.UUID(u)
}

// MarshalGQL implements the graphql.Marshaler interface
func (u UUID) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, `"`+uuid.UUID(u).String()+`"`)
}

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (u *UUID) UnmarshalGQL(v interface{}) error {
	switch v := v.(type) {
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return fmt.Errorf("invalid UUID format: %v", err)
		}
		*u = UUID(parsed)
		return nil
	default:
		return fmt.Errorf("UUID must be a string")
	}
}

// MarshalUUID marshals a uuid.UUID to GraphQL
func MarshalUUID(u uuid.UUID) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = io.WriteString(w, `"`+u.String()+`"`)
	})
}

// UnmarshalUUID unmarshals a uuid.UUID from GraphQL
func UnmarshalUUID(v interface{}) (uuid.UUID, error) {
	switch v := v.(type) {
	case string:
		return uuid.Parse(v)
	default:
		return uuid.Nil, fmt.Errorf("UUID must be a string")
	}
}

// DateTime scalar for GraphQL (RFC3339 format)
type DateTime time.Time

func (d DateTime) Time() time.Time {
	return time.Time(d)
}

// MarshalGQL implements the graphql.Marshaler interface
func (d DateTime) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, `"`+time.Time(d).Format(time.RFC3339)+`"`)
}

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (d *DateTime) UnmarshalGQL(v interface{}) error {
	switch v := v.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// Try ISO8601 without timezone
			parsed, err = time.Parse("2006-01-02T15:04:05", v)
			if err != nil {
				return fmt.Errorf("invalid DateTime format: %v", err)
			}
		}
		*d = DateTime(parsed)
		return nil
	default:
		return fmt.Errorf("DateTime must be a string")
	}
}

// MarshalDateTime marshals a time.Time to GraphQL
func MarshalDateTime(t time.Time) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = io.WriteString(w, `"`+t.Format(time.RFC3339)+`"`)
	})
}

// UnmarshalDateTime unmarshals a time.Time from GraphQL
func UnmarshalDateTime(v interface{}) (time.Time, error) {
	switch v := v.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// Try ISO8601 without timezone
			t, err = time.Parse("2006-01-02T15:04:05", v)
		}
		return t, err
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(i, 0), nil
	case int64:
		return time.Unix(v, 0), nil
	default:
		return time.Time{}, fmt.Errorf("DateTime must be a string or number")
	}
}
