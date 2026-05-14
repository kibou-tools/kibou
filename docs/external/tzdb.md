# Timezone database

The [Theory and pragmatics of the tz code and data][iana-tzdb-theory]
in the IANA docs provide a bunch of information
about how the timezone database is structured.

This doc captures the key information that we rely on
in the code, for easier cross-referencing of particular points.

## Choice of calendar

> The tz database models time using the proleptic Gregorian calendar
> with days containing 24 equal-length hours numbered 00 through 23,
> except when clock transitions occur.

## References

- IANA, "[Theory and pragmatics of the tz code and data][iana-tzdb-theory]."
  ([archived][iana-tzdb-theory-a], 2025-08-22)

[iana-tzdb-theory]: <https://data.iana.org/time-zones/tzdb/theory.html>
[iana-tzdb-theory-a]: <https://web.archive.org/web/20250822080506/https://data.iana.org/time-zones/tzdb/theory.html>
