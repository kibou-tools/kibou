# Quirks in third-party dependencies

This folder collects quirks encountered in 
third-party tools and libraries
that contributed to or directly caused
a problem in local development, CI or production.

"Quirks" are interpreted broadly;
all of the following count:

- Surprising undocumented behavior
- Surprising documented behavior
- Unexplained design decisions
- Bugs

This idea is based on Dan Luu's post
on the [Normalization of Deviance](https://danluu.com/wat/).

> People don't automatically know what should be normal,
> and when new people are onboarded,
> they can just as easily learn deviant processes
> that have become normalized as reasonable processes.
>
> Julia Evans described to me how this happens:
>
> new person joins
> new person: WTF WTF WTF WTF WTF
> old hands: yeah we know we're concerned about it
> new person: WTF WTF wTF wtf wtf w...
> new person gets used to it
> new person #2 joins
> new person #2: WTF WTF WTF WTF
> new person: yeah we know. we're concerned about it.
