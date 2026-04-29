package timezone

import "time"

// JST is the Japan Standard Time location (UTC+9).
var JST = time.FixedZone("JST", 9*60*60)
