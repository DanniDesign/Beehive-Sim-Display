package simulation

//go:generate stringer -type=State,Event

type State int
type Event int

const (
	// --- Common States ---
	StateWandering State = iota
	StateSick

	// --- Forager States ---
	StateMovingToExit
	StateOutside
	StateBringingHoneyToStorage

	// --- Queen States ---
	// StateCarryingHoney

	// --- Queen Attendant States ---
	StateCollectingHoneyFromStorage
	StateCarryingHoney
	StateSupplyingHoneyToQueen
)

const (
	// --- Common Events ---
	EventWander Event = iota
	EventSickness
	EventSicknessOver

	// --- Forager Events ---
	EventHoneySupplyLow
	EventWentOutside
	EventHarvestedHoney
	EventStoredHoney
	EventHoneySupplyOK

	// --- Queen Events ---
	EventReceivedHoney
	EventEggPlaced

	// --- Queen Attendant Events ---
	EventNoHoneyOnHand
	EventHoneyNeeded
	EventGettingHoney
	EventCollectedHoney
	EventBringingToQueen
	EventDeliveredHoney
)
