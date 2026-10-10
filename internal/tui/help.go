package tui

import "strings"

// helpText explains how to read the numbers. The approximations and the
// volume-is-not-value caveat must live here, per the metric definitions.
const helpText = `How to read blamey

Ownership      Lines that still exist at the blamed revision, attributed by
               git blame (-w -M -C, so whitespace changes and moved code do
               not steal authorship). It ignores --since/--until.
Bus factor     The fewest people whose combined ownership exceeds 50%.
Silo (!)       A file where one person owns over 80% and has not committed for
               six months or more.
Single owner * A file with exactly one author left.
~ approximate  Churn is lines added minus lines surviving. It counts code that
               anyone removed, not only its author.
Survival rate  Surviving lines divided by lines added. Not meaningful with
               --since, because ownership is not windowed.
Excluded       Vendored and generated code, lockfiles and binaries, unless you
               pass --include.

Line counts measure volume, not value.`

func (m *Model) helpView() string {
	m.help.SetWidth(m.w)
	var b strings.Builder
	b.WriteString(m.st.Section.Render("Help") + "\n\n")
	b.WriteString(helpText)
	b.WriteString("\n\n")
	b.WriteString(m.help.FullHelpView(m.km.FullHelp()))
	return b.String()
}
