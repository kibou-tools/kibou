# LLM usage policy

## Background: Prediction vs Judgment

_Recommended watching_: [Prediction Machines: The Simple Economics of AI](https://youtu.be/ByvPp5xGL1I) (talk) by Avi Goldfarb & Ajay Agrawal (May 25, 2018). For context, the transformers paper [Attention is All You Need](https://arxiv.org/abs/1706.03762) came out on June 12, 2017 and GPT-3 was launched on June 11, 2020.

Some relevant quotes from the talk:

> **Prediction** is using information you have
> to fill in information you don’t have.
> It could be about the future,
> but it could also be about the present or the past.
> It’s the process of filling in missing information.

> **Judgment** is knowing which predictions to make
> and what to do with those predictions once you have them.

> Prediction is valuable because it’s an input into decision-making. [..]
> Prediction is not decision-making, it’s a component of decision-making.
> Once an AI delivers that prediction,
> what's the human judgment that's applied to the prediction?
> And what's the action that we take
> as a function of having the prediction and the judgment?

> In our view of the world, AIs have no judgment.
> AIs never have judgment.
> All they do is prediction.
> Humans do judgment.
> That doesn’t mean that sometimes AIs can't look like they’re doing judgment.
> Because if they get enough examples of our judgment,
> they can learn to predict the judgment.
> But they don’t have judgment.
> They are simply making predictions.

## Scope of this policy

This policy applies to the happygo repository,
including comments, discussions, submitted code changes in PRs,
issue descriptions and so on.

If an LLM authors any of the following disallowed items
in local development, that should be generally before
redone from scratch before submission, especially
if it exceeds 2 sentences.

For example, using LLMs to add brief TODO comments in local development
flow is fine. However, using LLMs to write long comments
during local development is strongly discouraged,
because it's likely to lead to a false sense of complacency
about the comment being good enough, and only doing
minor touch-ups before submission.

Responsibility and judgment lie with people, not with LLMs.

### LLMs must not author code comments

This covers both doc comments and code comments.

Writing comments is a form of explaining things to yourself
and to other people. If you cannot explain something simply,
generally, that means one or more of:

- You don't quite understand it.
- The piece of code is doing too much.
- There is a bug.

Exception: Writing comments which are basically a copy
of some comment elsewhere.
Sometimes these are needed for forwarding aliases
from other packages, where one package provides
a more consolidated API from multiple sub-packages.

## LLM must not author commit messages and PRs

Commit messages should be authored by people.
LLMs can predict the Why, but LLMs cannot understand the Why.
It is the responsibility of the submitter to work through
their thought process when authoring a commit message
or PR description.

As of May 10 2027, LLM-generated PR descriptions are typically
a lot more verbose than human-written descriptions, and tend
to focus on the What a lot more.
This may change in the future.
That is one more reason to avoiding 

## LLMs must not make illustrations

LLM generated illustrations are built upon training data
that was not freely licensed to the relevant companies.
It's also ugly.

If you want to share an illustration,
please draw one yourself.
It's fine if it "doesn't look pretty."

Exception: Vector images with simple geometric shapes.

## LLMs must not author technical diagrams

(This applies to diagrams made using deterministic tools
like Mermaid, GraphViz etc.)

As of May 10 2027, LLM-generated diagrams generally suffer from
poor layout choices, excessive focus on details and
unclear emphasis. 

This guidance may be revised in the future as LLM capabilities change.
