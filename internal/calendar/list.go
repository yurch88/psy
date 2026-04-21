package calendar

import "sort"

func (s *Service) Bookings() ([]Booking, error) {
	bookings, err := s.store.List()
	if err != nil {
		return nil, err
	}

	sort.Slice(bookings, func(i, j int) bool {
		if bookings[i].CreatedAt.Equal(bookings[j].CreatedAt) {
			return bookings[i].Start.After(bookings[j].Start)
		}
		return bookings[i].CreatedAt.After(bookings[j].CreatedAt)
	})

	return bookings, nil
}

func (s *Store) List() ([]Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bookings, err := s.loadLocked()
	if err != nil {
		return nil, err
	}

	return append([]Booking(nil), bookings...), nil
}
