package queenattendant

import (
	"context"

	s "github.com/DanniDesign/Beehive-Sim-Display/simulation"
	"github.com/gookit/slog"
	"github.com/qmuntal/stateless"
)

type QueenAttendantFSM struct {
	fsm *stateless.StateMachine
	bee *s.Bee
}

func New(b *s.Bee) *stateless.StateMachine {
	qa := QueenAttendantFSM{
		bee: b,
		fsm: stateless.NewStateMachine(s.StateWandering),
	}
	qa.init()
	return qa.fsm
}

func (qa *QueenAttendantFSM) init() {
	qa.fsm.Configure(s.StateWandering).
		Permit(s.EventNoHoneyOnHand, s.StateCollectingHoneyFromStorage).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventDeliveredHoney).
		Ignore(s.EventCollectedHoney).
		Ignore(s.EventSicknessOver).
		Ignore(s.EventWander)
	qa.fsm.Configure(s.StateCollectingHoneyFromStorage).
		Permit(s.EventCollectedHoney, s.StateSupplyingHoneyToQueen).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventHoneyNeeded).
		Ignore(s.EventWander).
		Ignore(s.EventSicknessOver).
		Ignore(s.EventNoHoneyOnHand)
	qa.fsm.Configure(s.StateCarryingHoney).
		Permit(s.EventHoneyNeeded, s.StateSupplyingHoneyToQueen).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventCollectedHoney).
		Ignore(s.EventDeliveredHoney).
		Ignore(s.EventWander).
		Ignore(s.EventSicknessOver)
	qa.fsm.Configure(s.StateSupplyingHoneyToQueen).
		Permit(s.EventDeliveredHoney, s.StateWandering).
		Permit(s.EventSickness, s.StateSick).
		Ignore(s.EventCollectedHoney).
		Ignore(s.EventHoneyNeeded).
		Ignore(s.EventWander).
		Ignore(s.EventSicknessOver)
	// NOTE: previously had Ignore(s.EventWander) registered twice here.
	// Two candidate handlers for the same trigger with no disambiguating
	// guard panics at Fire-time — "Multiple permitted exit transitions...
	// Guard clauses must be mutually exclusive." Each trigger is now
	// registered exactly once for this state.
	qa.fsm.Configure(s.StateSick).
		Permit(s.EventSicknessOver, s.StateSupplyingHoneyToQueen, func(ctx context.Context, args ...any) bool {
			if len(args) == 0 {
				slog.Fatalf("FSM guard failed: no arguments provided")
				return false
			}
			bee, ok := args[0].(*s.Bee)
			if !ok {
				slog.Fatalf("Failed to parse bee argument in sickness state fsm.")
				return false
			}
			return bee.Carrying
		}).
		Permit(s.EventSicknessOver, s.StateWandering, func(ctx context.Context, args ...any) bool {
			if len(args) == 0 {
				slog.Fatalf("FSM guard failed: no arguments provided")
				return false
			}
			bee, ok := args[0].(*s.Bee)
			if !ok {
				slog.Fatalf("Failed to parse bee argument in sickness state fsm.")
				return false
			}
			return !bee.Carrying
		}).
		Ignore(s.EventHoneyNeeded).
		Ignore(s.EventWander).
		Ignore(s.EventCollectedHoney)
	qa.fsm.OnTransitioned(func(ctx context.Context, transition stateless.Transition) {
		slog.Infof("FSM Transitioned: %v -> %v (Trigger: %v)", transition.Source, transition.Destination, transition.Trigger)
	})

	qa.fsm.OnUnhandledTrigger(func(ctx context.Context, state stateless.State, trigger stateless.Trigger, unmetGuards []string) error {
		slog.Warnf("FSM Ignored/Unhandled Trigger! State: %v, Trigger: %v", state, trigger)
		return nil // Return nil so it doesn't crash, just logs
	})
}

func (qa *QueenAttendantFSM) State() s.State {
	return qa.fsm.MustState().(s.State)
}

func (qa *QueenAttendantFSM) Fire(ctx context.Context, event s.Event, args ...any) error {
	return qa.fsm.FireCtx(ctx, event, args...)
}
