# Project Governance

- [Values](#values)
- [Maintainers](#maintainers)
- [Becoming a Maintainer](#becoming-a-maintainer)
- [Meetings](#meetings)
- [Code of Conduct Enforcement](#code-of-conduct)
- [Voting](#voting)

## Values

The Kured project and its leadership embrace the following values:

- Openness: Communication and decision-making happens in the open and is discoverable for future
  reference. As much as possible, all discussions and work take place in public
  forums and open repositories.

- Fairness: All stakeholders have the opportunity to provide feedback and submit
  contributions, which will be considered on their merits.

- Community over Product or Company: Sustaining and growing our community takes
  priority over shipping code or sponsors' organizational goals.  Each
  contributor participates in the project as an individual.

- Inclusivity: We innovate through different perspectives and skill sets, which
  can only be accomplished in a welcoming and respectful environment.

- Participation: Responsibilities within the project are earned through
  participation, and there is a clear path up the contributor ladder into leadership
  positions.

- Consensus: Whether or not wider input is required, the Kured community believes that
  the best decisions are reached through Consensus
  <https://en.wikipedia.org/wiki/Consensus_decision-making>.

## Maintainers

Kured Maintainers have write access to the [project GitHub
organisation](https://github.com/kubereboot). They can merge their own patches or patches
from others. The current maintainers can be found in [MAINTAINERS][maintainers-file].
Maintainers collectively manage the project's resources and contributors.

This privilege is granted with some expectation of responsibility: maintainers
are people who care about the Kured project and want to help it grow and
improve. A maintainer is not just someone who can make changes, but someone who
has demonstrated their ability to collaborate with the team, get the most
knowledgeable people to review code and docs, contribute high-quality code, and
follow through to fix issues (in code or tests).

A maintainer is a contributor to the project's success and a citizen helping
the project succeed.

## Becoming a Maintainer

To become a Maintainer you need to demonstrate the following:

- commitment to the project:
  - participate in discussions, contributions, code and documentation reviews
      for 3 months or more and participate in Slack discussions and meetings
      if possible,
  - perform reviews for 5 non-trivial pull requests,
  - contribute 5 non-trivial pull requests and have them merged,
- ability to write quality code and/or documentation,
- ability to collaborate with the team,
- understanding of how the team works (policies, processes for testing and code review, etc),
- understanding of the project's code base and coding and documentation style.

We realise that everybody brings different abilities and qualities to the team, that's
why we are willing to change the rules somewhat depending on the circumstances.

A new Maintainer can apply by proposing a PR to the [MAINTAINERS
file][maintainers-file]. A simple majority vote of existing Maintainers
approves the application.

Maintainers who are selected will be granted the necessary GitHub rights,
and invited to the [private maintainer mailing list][private-list].

## Meetings

Time zones permitting, Maintainers are expected to participate in the public
developer meeting, details can be found [here][meeting-agenda].

Maintainers will also have closed meetings in order to discuss security reports
or Code of Conduct violations.  Such meetings should be scheduled by any
Maintainer on receipt of a security issue or CoC report.  All current Maintainers
must be invited to such closed meetings, except for any Maintainer who is
accused of a CoC violation.

## Code of Conduct

[Code of Conduct](./CODE_OF_CONDUCT.md) violations by community members will
be discussed and resolved on the [private Maintainer mailing list][private-list].
If the reported CoC violator is a Maintainer, the Maintainers will instead
designate two Maintainers to work with CNCF staff in resolving the report.

## Voting

While most business in Kured is conducted by "lazy consensus", periodically
the Maintainers may need to vote on specific actions or changes.
A vote can be taken in [kured issues labeled 'decision'][decision-issues] or
[the private Maintainer mailing list][private-list] for security or conduct
matters. Votes may also be taken at [the developer meeting][meeting-agenda].
Any Maintainer may demand a vote be taken.

Most votes require a simple majority of all Maintainers to succeed. Maintainers
can be removed by a 2/3 majority vote of all Maintainers, and changes to this
Governance require a 2/3 vote of all Maintainers.

[maintainers-file]: ./MAINTAINERS
[private-list]: cncf-kured-maintainers@lists.cncf.io
[meeting-agenda]: https://docs.google.com/document/d/1AWT8YDdqZY-Se6Y1oAlwtujWLVpNVK2M_F_Vfqw06aI/edit
[decision-issues]: https://github.com/kubereboot/kured/labels/decision
