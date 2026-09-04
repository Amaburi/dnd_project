package combat

import "time"

// nowUTC is the single clock the tracker uses, so a test can see one
// consistent notion of "now" across a whole encounter.
func nowUTC() time.Time { return time.Now().UTC() }
