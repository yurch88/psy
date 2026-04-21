package calendar

import (
	"context"
	"errors"
)

func (s *Service) Cancel(ctx context.Context, bookingID string) (Booking, error) {
	return s.store.Cancel(ctx, bookingID, s.now().UTC())
}

func (s *Service) Reschedule(ctx context.Context, bookingID, newSlotID string) (Booking, error) {
	slot, ok := s.findSlotExcluding(newSlotID, bookingID)
	if !ok {
		return Booking{}, ErrSlotNotFound
	}

	booking, err := s.store.Reschedule(ctx, bookingID, slot, s.now().UTC())
	if err != nil {
		if errors.Is(err, ErrSlotAlreadyTaken) {
			return Booking{}, err
		}
		return Booking{}, err
	}

	return booking, nil
}
