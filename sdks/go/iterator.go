package ghanageo

import "context"

type PageIterator[T any] struct {
	fetch         func(context.Context, string) (Page[T], error)
	next          string
	current       Page[T]
	err           error
	started, done bool
	seen          map[string]struct{}
}

func NewPageIterator[T any](fetch func(context.Context, string) (Page[T], error)) *PageIterator[T] {
	return &PageIterator[T]{fetch: fetch, seen: map[string]struct{}{}}
}
func (i *PageIterator[T]) Next(ctx context.Context) bool {
	if i.done || i.err != nil {
		return false
	}
	if i.started && i.next == "" {
		i.done = true
		return false
	}
	i.started = true
	p, err := i.fetch(ctx, i.next)
	if err != nil {
		i.err = err
		return false
	}
	if p.NextCursor != "" {
		if _, ok := i.seen[p.NextCursor]; ok {
			i.err = ErrRepeatedCursor
			return false
		}
		i.seen[p.NextCursor] = struct{}{}
	}
	i.current = p
	i.next = p.NextCursor
	return true
}
func (i *PageIterator[T]) Page() Page[T] { return i.current }
func (i *PageIterator[T]) Err() error    { return i.err }
func (c *Client) RegionPages(o PageOptions) *PageIterator[Region] {
	o.Cursor = ""
	return NewPageIterator(func(ctx context.Context, cursor string) (Page[Region], error) {
		o.Cursor = cursor
		return c.Regions(ctx, o)
	})
}
func (c *Client) DistrictPages(o DistrictOptions) *PageIterator[District] {
	o.Cursor = ""
	return NewPageIterator(func(ctx context.Context, cursor string) (Page[District], error) {
		o.Cursor = cursor
		return c.Districts(ctx, o)
	})
}
func (c *Client) PlacePages(o PlaceOptions) *PageIterator[Place] {
	o.Cursor = ""
	return NewPageIterator(func(ctx context.Context, cursor string) (Page[Place], error) {
		o.Cursor = cursor
		return c.Places(ctx, o)
	})
}
