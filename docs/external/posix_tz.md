# POSIX Timezone format

POSIX supports the `TZ` environment variable to represent
"timezone information." -- search for "timezone information"
in the [POSIX Issue 8 Ch. 8 Environment Variables][posix8-ch8-env-vars].

The spec's HTML unfortunately doesn't allow making permalinks
to different subsections, so I've copied the key points and
my interpretations here, for references from the code.

Basically, the `TZ` env var can be one of 3 forms.

## Implementation-defined form

```
:characters
```

> If TZ is of the first format (that is, if the first character is a <colon>),
> the characters following the <colon> are handled in an implementation-defined manner.

## Rule-based form

```
stdoffset[dst[offset][,start[/time],end[/time]]]
                     ^-------------------------^
                               rule

Examples:
TODO(add examples here)
```

### Name format

- `std` and `dst` (if present) have length `l ∈ [3, TZNAME_MAX]`.
  Based on local testing, the values in practice are:
  - `getconf TZNAME_MAX` returns 255 on macOS 15.7.4
  - `getconf TZNAME_MAX` returns undefined/unset on Debian 13
  - `getconf TZNAME_MAX` returns 6 on Alpine 3.23

These have two forms, quoted and unquoted.

#### Quoted form

For quoted, the spec says:

> In the quoted form, the first character shall be the <less-than-sign> ('<') character
> and the last character shall be the <greater-than-sign> ('>') character. All characters
> between these quoting characters shall be alphanumeric characters from the portable
> character set in the current locale, the <plus-sign> ('+') character, or the
> <hyphen-minus> ('-') character. The std and dst fields in this case shall not include
> the quoting characters and the quoting characters do not contribute to the three byte
> minimum length and {TZNAME_MAX} maximum length.

The wording is potentially a bit confusing: the "alphanumeric characters from the portable
character set in the current locale" does not imply that the set of allowed characters is
locale-dependent.

Per [POSIX Issue 8 Ch. 6 Character Set][posix8-ch6-char-set]:

> Each supported locale shall include the portable character set, which is the set of symbolic names for characters in Portable Character Set
> (table of characters)

The full portable character set is essentially a subset of the code point
range `[U+0000 (NUL), U+007E (~)]`. Since only alphanumeric characters
matter here, it's just a fancy way of saying `[A-Za-z0-9]`.

The "tricky" thing is encoding. A codepoint may be encoded as UTF-8, [EBCDIC][wikipedia-ebcdic]
or something else. In theory, there's nothing stopping you from providing
env vars with whatever encoding you want.

In practice, we only care about Linux and macOS (amongst POSIX-y environments),
where existing processes generally expect ASCII/UTF-8 encoding for env vars.

#### Unquoted form

> In the unquoted form, all characters in these fields shall be
> alphabetic characters from the portable character set in the current locale.

Similar to the previous subsection, this means `[A-Za-z]`.

### Offset format

> offset - Indicates the value added to the local time to arrive at Coordinated Universal Time.
> The offset has the form:
> 
> hh[:mm[:ss]]
> 
> The minutes (mm) and seconds (ss) are optional. The hour (hh) shall be required and may be a single digit.
> [..]
> One or more digits may be used; the value is always interpreted as a decimal number.
> The hour shall be between zero and 24, and the minutes (and seconds)—if present—between zero and 59.
> The result of using values outside of this range is unspecified.
> If preceded by a '-', the timezone shall be east of the Prime Meridian;
> otherwise, it shall be west (which may be indicated by an optional preceding '+').

The convention is the opposite to that of common parlance.
E.g. Indian Standard Time (IST) is UTC+05:30, but since the description
says "added to the local time to arrive at (UTC)", IST would be written
as -5:30 or -05:30 here.

#### Daylight Saving Time default offset

> If no offset follows dst, Daylight Saving Time is assumed to be one hour ahead of standard time.

This needs special care.

### Rule format

> Indicates when to change from standard time to Daylight Saving Time,
> and when to change back. The rule has the form:
> 
> date\[/time\],date\[/time\]
>
> where the first date describes when the change
> from standard time to Daylight Saving Time
> occurs and the second date describes when it ends;
> \[..\]
> Each time field describes when, in current local time,
> the change to the other time is made.

#### Rule date order

> if the second date is specified as earlier in the year than the first,
> then the year begins and ends in Daylight Saving Time.

#### Rule date format

> The format of date is one of the following:
>
> - Jn: The Julian day n (1 <= n <= 365). Leap days shall not be counted.
>   That is, in all years—including leap years—February 28 is day 59 and
>   March 1 is day 60. It is impossible to refer explicitly to the
>   occasional February 29.
> - n: The zero-based Julian day (0 <= n <= 365).
>   Leap days shall be counted, and it is possible to refer to February 29.
> - Mm.n.d: The d'th day (0 <= d <= 6) of week n of month m of the year
>   (1 <= n <= 5, 1 <= m <= 12, where week 5 means
>   "the last d day in month m" which may occur in either the fourth or the fifth week).
>   Week 1 is the first week in which the d'th day occurs. Day zero is Sunday.

While the POSIX docs don't mention the calendar, the [IANA docs][iana-tzdb-theory] state:

> In POSIX, time display in a process is controlled by the environment variable TZ,
> which can have two forms:
>
> - A proleptic TZ value like CET-1CEST,M3.5.0,M10.5.0/3 \[..\].
> - A geographical TZ value like Europe/Berlin \[..\]

This clarifies that the TZ value should be interpreted based on the proleptic Gregorian calendar. 

#### Rule time format

> The time has the same format as offset except that
> the hour can range from zero to 167.
> If preceded by a '-', the time shall count backwards before midnight.
> For example, "47:30" stands for 23:30 the next day,
> and "-3:30" stands for 20:30 the previous day.
> The default, if time is not given, shall be 02:00:00.

## IANA timezone

> A format specifying a geographical timezone or a special timezone.

## Overview

> the value of TZ is in one of the three formats (spaces inserted for clarity):
>
> :characters
>
> or:
>
> std offset dst offset, rule
>
> or:
>
> A format specifying a geographical timezone or a special timezone.

This 

## References

- The Open Group, "[POSIX.1-2024, Issue 8, Chapter 8: Environment
  Variables][posix8-ch8-env-vars]."
  ([archived][posix8-ch8-env-vars-a], 2026-05-01)

- The Open Group, "[POSIX.1-2024, Issue 8, Chapter 6: Character
  Set][posix8-ch6-char-set]."
  ([archived][posix8-ch6-char-set-a], 2025-11-18)

- IANA, "[Theory and pragmatics of the tz code and data][iana-tzdb-theory]."
  ([archived][iana-tzdb-theory-a], 2025-08-22)

- Wikipedia, "[EBCDIC][wikipedia-ebcdic]."
  (accessed 2026-05-16)

[wikipedia-ebcdic]: <https://en.wikipedia.org/wiki/EBCDIC>
[posix8-ch6-char-set]: <https://pubs.opengroup.org/onlinepubs/9799919799.2024edition/basedefs/V1_chap06.html>
[posix8-ch6-char-set-a]: <https://web.archive.org/web/20251118165826/https://pubs.opengroup.org/onlinepubs/9799919799.2024edition/basedefs/V1_chap06.html>
[posix8-ch8-env-vars]: <https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap08.html>
[posix8-ch8-env-vars-a]: <https://web.archive.org/web/20260501232928/https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap08.html>
[iana-tzdb-theory]: <https://data.iana.org/time-zones/tzdb/theory.html>
[iana-tzdb-theory-a]: <https://web.archive.org/web/20250822080506/https://data.iana.org/time-zones/tzdb/theory.html>
