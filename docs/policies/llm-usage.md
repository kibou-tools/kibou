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

This policy applies to the kibou repository,
including comments, discussions, submitted code changes in PRs,
issue descriptions and so on.

This policy does not apply to personal usage locally.
Some of the rules are enforced using the LLM configuration
in the repository. You may override them locally
for personal use.

## Rules

The following usages of LLMs are permitted:

- Background research.
- Investigating and debugging issues.
- Using autocomplete for code, comments,
  simple vector art, or technical diagrams.
- Refactoring, migrations, and other more mechanical changes
  which do not involve major API-design decisions,
  but reuse existing decisions.

When citing LLM-generated text in an issue description
or PR, specify the model used, and clearly identify
said text using blockquotes or annotated code blocks.

All other usages are forbidden in shared contexts.
For example, LLMs must not be used for:

- Authoring code for merging.
- Authoring code comments for merging.
- Authoring commit messages or PR descriptions.
- Creating illustrations/bitmap images.

If you personally use LLMs locally for these purposes,
you must redo the work by hand or not submit it.

Responsibility and judgment lie with people, not with LLMs.

## Q&A

### Why is agentic LLM usage forbidden for shared code?

Over the period of December 2025 to June 2026,
I've used LLMs for generating a fair bit of code.
Generally, LLM-generated code requires many more
rounds of code review compared to human-written code.

After a few rounds of review,
it's easy to accidentally fall into
a false sense of complacency when reviewing changes,
and assume that certain "basic" changes are correct
without careful review.
However, the jagged nature of LLM capabilities
means that they can fail at doing basic tasks,
while succeeding at trickier tasks.

Additionally, it's easy to lull oneself into thinking
that one has understood the domain and the solution,
but when the code actually falls over later,
you end up realizing that you didn't actually understand either.

From the point-of-view of a language toolchain
that is meant to be core infrastructure,
it doesn't make sense to trade off the increased risk of bugs
for speed of implementation.

### Why is agentic LLM usage forbidden for code comments?

Writing comments is a form of explaining things to yourself
and to other people. If you cannot explain something simply,
generally, that means one or more of:

- You don't quite understand it.
- The piece of code is doing too much.
- There is a bug.

### Why is agentic LLM usage forbidden for commit messages and PRs?

Commit messages should be authored by people.
LLMs can predict the Why, but LLMs cannot understand the Why.
It is the responsibility of the submitter to work through
their thought process when authoring a commit message
or PR description.

### Why is LLM usage forbidden for illustrations?

LLM generated illustrations are built upon training data
that was not freely licensed to the relevant companies.
It's also ugly.

If you want to share an illustration,
please draw one yourself.
It's fine if it "doesn't look pretty."

### Why is LLM usage forbidden for technical diagrams?

(This applies to diagrams made using deterministic tools
like Mermaid, GraphViz etc.)

As of May 10 2026, LLM-generated diagrams generally suffer from
poor layout choices, excessive focus on details and
unclear emphasis. 

This guidance may be revised in the future as LLM capabilities change.
