# Beehive-Sim-Display

A honey bee colony simulation shown on a 64x32 RGB LED matrix (Raspberry Pi,
`-tags matrix`) or in a desktop preview window (ebiten).

## What is simulated

The model follows real *Apis mellifera* biology, scaled down to a 64x32 comb:

* **Calendar** – a real 365-day year (day 0 = 1 March) with temperate
  climate curves for temperature, day length, rain and forage (willow,
  fruit blossom, rapeseed, the "June gap", lime/clover, heather, ivy).
  One day is 180 ticks (36 s at 5 FPS), so a full year takes ~3.6 hours.
  Cells dim at night.
* **Brood** – eggs (3 days) → larvae (6 days, must be fed by nurses or they
  starve) → capped pupae (12 days; drones 14.5, queens 7.5) → adult.
  Emerged cells must be cleaned before the queen lays in them again.
* **Division of labour** – workers progress cleaner → nurse → house bee /
  guard → forager with age. The pace adapts to the colony: few foragers
  speeds it up, little brood slows it down (long-lived *winter bees*),
  and too few nurses makes young foragers revert to nursing.
* **Food** – foragers fly out in good weather, bring back nectar or pollen
  (more pollen when there is open brood) and do waggle dances that recruit
  more foragers. Nectar ripens and is capped as honey by house bees. All
  bees and larvae eat honey continuously; nurses eat pollen to make brood
  food. A colony that runs out starves.
* **Comb layout** – pollen is packed around the brood, nectar just beyond it
  and above, producing the classic honey arch. Bees eat from the inner edge
  of the arch first. In winter the colony forms a tight cluster that slowly
  creeps towards its honey.
* **Queen** – lays up to 16 eggs/day following the season, throttled by food
  stores and nurse availability, and is fed by a retinue of nurses (red).
* **Drones** – reared April–August, make mating flights, and are thrown out
  in late summer or when food is short.
* **Swarming** – a strong, crowded colony in spring raises queen cells; when
  the first is capped the old queen leaves with ~60% of the workers. The
  first virgin queen to emerge kills her rivals and goes on mating flights
  (she may be lost).
* **Queen loss** – an emergency queen is raised from a young larva if there
  is one; otherwise laying workers produce only drones and the colony dies.
  An empty hive is robbed and later re-occupied by a wild swarm.

## Colours

| Pixel | Meaning |
|---|---|
| magenta | queen |
| red | nurse feeding the queen |
| grey / white | young house bees / older bees |
| yellow, orange | bee carrying nectar / pollen |
| flashing yellow-white | waggle dance |
| blue | drone |
| off-white / green / brown | egg / larva (grows brighter) / capped brood |
| purple | queen cell |
| pale gold / bright gold | nectar / capped honey |
| orange-red | pollen |
| cyan | entrance |

## Running

```sh
go run .                                  # desktop preview
./deploy.sh                               # cross-compile for the Pi and deploy
```

## Testing the model

```sh
go test ./simulation                      # invariants over many colonies, seasonal checks
go run ./cmd/montecarlo -runs 1000 -years 3   # Monte Carlo statistics
go run ./cmd/montecarlo -trace -seed 7        # one colony, day by day
```

The simulation is deterministic for a given seed (`simulation.NewHiveWithSeed`),
and all tuning knobs live in `simulation.Params`.
