// Copyright (c) 2019, The Decred developers
// See LICENSE for details.

package dcrpg

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/decred/dcrdata/db/dcrpg/internal"
)

// parseUnit is used to separate a "unit" from pg_settings such as "8kB" into a
// numeric component and a base unit string.
func parseUnit(unit string) (multiple float64, baseUnit string, err error) {
	// This regular expression is defined so that it will match any input.
	re := regexp.MustCompile(`([-\d\.]*)\s*(.*)`)
	matches := re.FindStringSubmatch(unit)
	// One or more of the matched substrings may be "", but the base unit
	// substring (matches[2]) will match anything.
	if len(matches) != 3 {
		panic("inconceivable!")
	}

	// The regexp eats leading spaces, but there may be trailing spaces
	// remaining that should be removed.
	baseUnit = strings.TrimSuffix(matches[2], " ")

	// The numeric component is processed by strconv.ParseFloat except in the
	// cases of an empty string or a single "-", which is interpreted as a
	// negative sign.
	switch matches[1] {
	case "":
		multiple = 1
	case "-":
		multiple = -1
	default:
		multiple, err = strconv.ParseFloat(matches[1], 64)
		if err != nil {
			// If the numeric part does not parse as a valid number (e.g.
			// "3.2.1-"), reset the base unit and return the non-nil error.
			baseUnit = ""
		}
	}

	return
}

// PGSetting describes a PostgreSQL setting scanned from pg_settings.
type PGSetting struct {
	Name, Setting, Unit, ShortDesc, Source, SourceFile, SourceLine string
}

// PGSettings facilitates looking up a PGSetting based on a setting's Name.
type PGSettings map[string]PGSetting

// String implements the Stringer interface, generating a table of the settings
// where the Setting and Unit fields are merged into a single column. The rows
// of the table are sorted by the PGSettings string key (the setting's Name).
// This function is not thread-safe, so do not modify PGSettings concurrently.
func (pgs PGSettings) String() string {
	// Sort the names.
	numSettings := len(pgs)
	names := make([]string, 0, numSettings)
	for name := range pgs {
		names = append(names, name)
	}
	sort.Strings(names)

	// Determine max width of "Setting", "Name", and "File" entries.
	fileWidth, nameWidth, settingWidth := 4, 4, 7
	// Also combine Setting and Unit, in the same order as the sorted names.
	fullSettings := make([]string, 0, numSettings)
	for i := range names {
		s, ok := pgs[names[i]]
		if !ok {
			log.Errorf("(PGSettings).String is not thread-safe!")
			continue
		}

		// Combine Setting and Unit.
		fullSetting := s.Setting
		// See if setting is numeric. Assume non-numeric settings have no Unit.
		if num1, err := strconv.ParseFloat(s.Setting, 64); err == nil {
			// Combine with the unit if numeric.
			if num2, unit, err := parseUnit(s.Unit); err == nil {
				if unit != "" {
					unit = " " + unit
				}
				// Combine. e.g. 10.0, "8kB" => "80 kB"
				fullSetting = fmt.Sprintf("%.12g%s", num1*num2, unit)
			} else {
				// Mystery unit.
				fullSetting += " " + s.Unit
			}
		}

		fullSettings = append(fullSettings, fullSetting)

		if len(fullSetting) > settingWidth {
			settingWidth = len(fullSetting)
		}

		// File column width.
		if len(s.SourceFile) > fileWidth {
			fileWidth = len(s.SourceFile)
		}
		// Name column width.
		if len(s.Name) > nameWidth {
			nameWidth = len(s.Name)
		}
	}

	format := "%" + strconv.Itoa(nameWidth) + "s | %" + strconv.Itoa(settingWidth) +
		"s | %10.10s | %" + strconv.Itoa(fileWidth) + "s | %5s | %-48.48s\n"

	// Write the headers and a horizontal bar.
	out := fmt.Sprintf(format, "Name", "Setting", "Source", "File", "Line", "Description")
	hbar := strings.Repeat(string([]rune{0x2550}), nameWidth+1) + string([]rune{0x256A}) +
		strings.Repeat(string([]rune{0x2550}), settingWidth+2) + string([]rune{0x256A}) +
		strings.Repeat(string([]rune{0x2550}), 12) + string([]rune{0x256A}) +
		strings.Repeat(string([]rune{0x2550}), fileWidth+2) + string([]rune{0x256A}) +
		strings.Repeat(string([]rune{0x2550}), 7) + string([]rune{0x256A}) +
		strings.Repeat(string([]rune{0x2550}), 50)
	out += hbar + "\n"

	// Write each row.
	for i := range names {
		s, ok := pgs[names[i]]
		if !ok {
			log.Warnf("(PGSettings).String is not thread-safe!")
			continue
		}
		out += fmt.Sprintf(format, s.Name, fullSettings[i], s.Source,
			s.SourceFile, s.SourceLine, s.ShortDesc)
	}
	return out
}

// RetrievePGVersion retrieves the version of the connected PostgreSQL server.
func RetrievePGVersion(db *sql.DB) (ver string, err error) {
	err = db.QueryRow(internal.RetrievePGVersion).Scan(&ver)
	return
}

// retrieveSysSettings retrieves the PostgreSQL settings provided a query that
// returns the following columns from pg_setting in order: name, setting, unit,
// short_desc, source, sourcefile, sourceline.
func retrieveSysSettings(stmt string, db *sql.DB) (PGSettings, error) {
	rows, err := db.Query(stmt)
	if err != nil {
		return nil, err
	}

	defer closeRows(rows)

	settings := make(PGSettings)

	for rows.Next() {
		var name, setting, unit, shortDesc, source, sourceFile sql.NullString
		var sourceLine sql.NullInt64
		err = rows.Scan(&name, &setting, &unit, &shortDesc,
			&source, &sourceFile, &sourceLine)
		if err != nil {
			return nil, err
		}

		// If the source is "configuration file", but the file path is empty,
		// the connected postgres user does not have sufficient privileges.
		var line, file string
		if source.String == "configuration file" {
			// Shorten the source string.
			source.String = "conf file"
			if sourceFile.String == "" {
				file = "NO PERMISION"
			} else {
				file = sourceFile.String
				line = strconv.FormatInt(sourceLine.Int64, 10)
			}
		}

		settings[name.String] = PGSetting{
			Name:       name.String,
			Setting:    setting.String,
			Unit:       unit.String,
			ShortDesc:  shortDesc.String,
			Source:     source.String,
			SourceFile: file,
			SourceLine: line,
		}
	}

	return settings, nil
}

// RetrieveSysSettingsConfFile retrieves settings that are set by a
// configuration file (rather than default, environment variable, etc.).
func RetrieveSysSettingsConfFile(db *sql.DB) (PGSettings, error) {
	return retrieveSysSettings(internal.RetrieveSysSettingsConfFile, db)
}

// RetrieveSysSettingsPerformance a performance-related settings.
func RetrieveSysSettingsPerformance(db *sql.DB) (PGSettings, error) {
	return retrieveSysSettings(internal.RetrieveSysSettingsPerformance, db)
}

// RetrieveSysSettingsServer a key server configuration settings (config_file,
// data_directory, max_connections, dynamic_shared_memory_type,
// max_files_per_process, port, unix_socket_directories), which may be helpful
// in debugging connectivity issues or other DB errors.
func RetrieveSysSettingsServer(db *sql.DB) (PGSettings, error) {
	return retrieveSysSettings(internal.RetrieveSysSettingsServer, db)
}
