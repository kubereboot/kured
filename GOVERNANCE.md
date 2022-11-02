# Project Governance

This is an example governance document for projects that use the very common
"maintainer council" system of governance.  See [GOVERNANCE.md](/GOVERNANCE.md)
for more information.  Thanks to the Jaeger Project for portions of the text
below.

This is a template document for CNCF projects that requires editing
before it is ready to use. Read the markdown comments, `<!-- COMMENT -->`, for
additional guidance. The raw markdown uses `TODO` to identify areas that
require customization.  Replace [TODO: PROJECTNAME] with the name of your project.

- [Values](#values)
- [Maintainers](#maintainers)
- [Becoming a Maintainer](#becoming-a-maintainer)
- [Meetings](#meetings)
- [CNCF Resources](#cncf-resources)
- [Code of Conduct Enforcement](#code-of-conduct)
- [Voting](#voting)

## Values

<!-- This is where you put the core values or principles of your project, like
openness, distributed design, fairness, diversity, etc.

References and Examples
* https://www.apache.org/theapacheway/
* https://kubernetes.io/community/values/

See https://contribute.cncf.io/maintainers/governance/charter for guidance and
additional examples.  The values below are just example values as a jumping-off
point for your project's actual values.  -->

The [TODO: PROJECTNAME] and its leadership embrace the following values:

* Openness: Communication and decision-making happens in the open and is discoverable for future
  reference. As much as possible, all discussions and work take place in public
  forums and open repositories.

* Fairness: All stakeholders have the opportunity to provide feedback and submit
  contributions, which will be considered on their merits.

* Community over Product or Company: Sustaining and growing our community takes
  priority over shipping code or sponsors' organizational goals.  Each
  contributor participates in the project as an individual.

* Inclusivity: We innovate through different perspectives and skill sets, which
  can only be accomplished in a welcoming and respectful environment.

* Participation: Responsibilities within the project are earned through
  participation, and there is a clear path up the contributor ladder into leadership
  positions.

## Maintainers

[TODO: PROJECTNAME] Maintainers have write access to the [project GitHub repository](TODO).
They can merge their own patches or patches from others. The current maintainers
can be found in [OWNERS](./OWNERS).  Maintainers collectively manage the project's
resources and contributors.

This privilege is granted with some expectation of responsibility: maintainers
are people who care about the [TODO: PROJECTNAME] project and want to help it grow and
improve. A maintainer is not just someone who can make changes, but someone who
has demonstrated their ability to collaborate with the team, get the most
knowledgeable people to review code and docs, contribute high-quality code, and
follow through to fix issues (in code or tests).

A maintainer is a contributor to the project's success and a citizen helping
the project succeed.

## Becoming a Maintainer

<!-- If you have full Contributor Ladder documentation that covers becoming
a Maintainer or Owner, then this section should instead be a reference to that
documentation -->

To become a Maintainer you need to demonstrate the following:

  * commitment to the project:
    * participate in discussions, contributions, code and documentation reviews
      for [TODO: Time Period] or more,
    * perform reviews for [TODO:Number] non-trivial pull requests,
    * contribute [TODO:Number] non-trivial pull requests and have them merged,
  * ability to write quality code and/or documentation,
  * ability to collaborate with the team,
  * understanding of how the team works (policies, processes for testing and code review, etc),
  * understanding of the project's code base and coding and documentation style.
  <!-- add any additional Maintainer requirements here -->

A new Maintainer must be proposed by an existing maintainer by sending a message to the
[developer mailing list](TODO: List Link). A simple majority vote of existing Maintainers
approves the application.

Maintainers who are selected will be granted the necessary GitHub rights,
and invited to the [private maintainer mailing list](TODO).

## Meetings

Time zones permitting, Maintainers are expected to participate in the public
developer meeting, which occurs
[TODO: Details of regular developer or maintainer meeting here].  

Maintainers will also have closed meetings in order to discuss security reports
or Code of Conduct violations.  Such meetings should be scheduled by any
Maintainer on receipt of a security issue or CoC report.  All current Maintainers
must be invited to such closed meetings, except for any Maintainer who is
accused of a CoC violation.

## CNCF Resources

Any Maintainer may suggest a request for CNCF resources, either in the
[mailing list](TODO: link to developer/maintainer mailing list), or during a
meeting.  A simple majority of Maintainers approves the request.  The Maintainers
may also choose to delegate working with the CNCF to non-Maintainer community
members.

## Code of Conduct

<!-- This assumes that your project does not have a separate Code of Conduct
Committee; most maintainer-run projects do not.  Remember to place a link
to the private Maintainer mailing list or alias in the code-of-conduct file.-->

[Code of Conduct](./code-of-conduct.md)
violations by community members will be discussed and resolved
on the [private Maintainer mailing list](TODO).  If the reported CoC violator
is a Maintainer, the Maintainers will instead designate two Maintainers to work
with CNCF staff in resolving the report.

## Voting

While most business in [TODO: PROJECTNAME] is conducted by "lazy consensus", periodically
the Maintainers may need to vote on specific actions or changes.
A vote can be taken on [the developer mailing list](TODO) or
[the private Maintainer mailing list](TODO) for security or conduct matters.  
Votes may also be taken at [the developer meeting](TODO).  Any Maintainer may
demand a vote be taken.

Most votes require a simple majority of all Maintainers to succeed. Maintainers
can be removed by a 2/3 majority vote of all Maintainers, and changes to this
Governance require a 2/3 vote of all Maintainers.
