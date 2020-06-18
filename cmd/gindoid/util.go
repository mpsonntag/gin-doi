package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	gdtmpl "github.com/G-Node/gin-doi/templates"
	"github.com/G-Node/libgin/libgin"
)

// Global function map for the templates that render the DOI information
// (request page and landing page).
var tmplfuncs = template.FuncMap{
	"Upper":            strings.ToUpper,
	"FunderName":       FunderName,
	"AwardNumber":      AwardNumber,
	"AuthorBlock":      AuthorBlock,
	"JoinComma":        JoinComma,
	"Replace":          strings.ReplaceAll,
	"FormatReferences": FormatReferences,
	"FormatCitation":   FormatCitation,
	"FormatIssuedDate": FormatIssuedDate,
	"KeywordPath":      KeywordPath,
	"FormatAuthorList": FormatAuthorList,
}

func readBody(r *http.Request) (*string, error) {
	body, err := ioutil.ReadAll(r.Body)
	x := string(body)
	return &x, err
}

func makeUUID(URI string) string {
	if doi, ok := libgin.UUIDMap[URI]; ok {
		return doi
	}
	currMd5 := md5.Sum([]byte(URI))
	return hex.EncodeToString(currMd5[:])
}

// EscXML runs a string through xml.EscapeText.
// This is a utility function for the doi.xml template.
func EscXML(txt string) string {
	buf := new(bytes.Buffer)
	if err := xml.EscapeText(buf, []byte(txt)); err != nil {
		log.Printf("Could not escape: %q :: %s", txt, err.Error())
		return ""
	}
	return buf.String()
}

// ReferenceDescription creates a string representation of a reference for use in the XML description tag.
// This is a utility function for the doi.xml template.
func ReferenceDescription(ref libgin.Reference) string {
	var namecitation string
	if ref.Name != "" && ref.Citation != "" {
		namecitation = ref.Name + " " + ref.Citation
	} else {
		namecitation = ref.Name + ref.Citation
	}

	if !strings.HasSuffix(namecitation, ".") {
		namecitation += "."
	}
	refDesc := fmt.Sprintf("%s: %s (%s)", ref.RefType, namecitation, ref.ID)
	return EscXML(refDesc)
}

// ReferenceSource splits the source type from a reference string of the form <source>:<ID>
// This is a utility function for the doi.xml template.
func ReferenceSource(ref libgin.Reference) string {
	idparts := strings.SplitN(ref.ID, ":", 2)
	if len(idparts) != 2 {
		// Malformed ID (no colon)
		// No source type
		return ""
	}
	return EscXML(idparts[0])
}

// ReferenceID splits the ID from a reference string of the form <source>:<ID>
// This is a utility function for the doi.xml template.
func ReferenceID(ref libgin.Reference) string {
	idparts := strings.SplitN(ref.ID, ":", 2)
	if len(idparts) != 2 {
		// Malformed ID (no colon)
		// No source type
		return EscXML(idparts[0])
	}
	return EscXML(idparts[1])
}

// FunderName splits the funder name from a funding string of the form <FunderName>, <AwardNumber>.
// This is a utility function for the doi.xml template.
func FunderName(fundref string) string {
	fuparts := strings.SplitN(fundref, ",", 2)
	if len(fuparts) != 2 {
		// No comma, return as is
		return EscXML(fundref)
	}
	return EscXML(strings.TrimSpace(fuparts[0]))
}

// AwardNumber splits the award number from a funding string of the form <FunderName>, <AwardNumber>.
// This is a utility function for the doi.xml template.
func AwardNumber(fundref string) string {
	fuparts := strings.SplitN(fundref, ",", 2)
	if len(fuparts) != 2 {
		// No comma, return empty
		return ""
	}
	return EscXML(strings.TrimSpace(fuparts[1]))
}

// AuthorBlock builds the author section for the landing page template.
// It includes a list of authors, their affiliations, and superscripts to associate authors with affiliations.
// This is a utility function for the landing page HTML template.
func AuthorBlock(authors []libgin.Creator) template.HTML {
	authorMap := make(map[*libgin.Creator]int) // maps Author -> Affiliation Number
	affiliationMap := make(map[string]int)     // maps Affiliation Name -> Affiliation Number
	affilNumberMap := make(map[int]string)     // maps Affiliation Number -> Affiliation Name (inverse of above)
	for _, author := range authors {
		if _, ok := affiliationMap[author.Affiliation]; !ok {
			// new affiliation; give it a new number
			number := 0
			// NOTE: adding the empty affiliation helps us figure out if a
			// single unique affiliation should be numbered, since we should
			// differentiate between authors that share the affiliation and the
			// ones that have none.
			if author.Affiliation != "" {
				number = len(affiliationMap) + 1
			} // otherwise it gets the "special" value 0
			affiliationMap[author.Affiliation] = number
			affilNumberMap[number] = author.Affiliation
		}
		authorMap[&author] = affiliationMap[author.Affiliation]
	}

	nameElements := make([]string, len(authors))
	// Format authors
	for idx, author := range authors {
		var url, id, affiliationSup string
		if author.Identifier != nil {
			id = author.Identifier.ID
			url = author.Identifier.SchemeURI + id
		}

		// Author names are LastName, FirstName
		namesplit := strings.SplitN(author.Name, ",", 2)
		name := author.Name
		// If there's no comma, just display as is
		if len(namesplit) == 2 {
			name = fmt.Sprintf("%s %s", strings.TrimSpace(namesplit[1]), strings.TrimSpace(namesplit[0]))
		}

		// Add superscript to name if it has an affiliation and there are more than one (including empty)
		if author.Affiliation != "" && len(affiliationMap) > 1 {
			affiliationSup = fmt.Sprintf("<sup>%d</sup>", affiliationMap[author.Affiliation])
		}

		nameElements[idx] = fmt.Sprintf("<span itemprop=\"author\" itemscope itemtype=\"http://schema.org/Person\"><a href=%q itemprop=\"url\"><span itemprop=\"name\">%s</span></a><meta itemprop=\"affiliation\" content=%q /><meta itemprop=\"identifier\" content=%q>%s</span>", url, name, author.Affiliation, id, affiliationSup)
	}

	// Format affiliations in number order (excluding empty)
	affiliationLines := "<ol class=\"doi itemlist\">\n"
	for idx := 1; ; idx++ {
		affiliation, ok := affilNumberMap[idx]
		if !ok {
			break
		}
		var supstr string
		if len(affiliationMap) > 1 {
			supstr = fmt.Sprintf("<sup>%d</sup>", idx)
		}
		affiliationLines = fmt.Sprintf("%s\t<li>%s%s</li>\n", affiliationLines, supstr, affiliation)
	}
	affiliationLines = fmt.Sprintf("%s</ol>", affiliationLines)

	authorLines := fmt.Sprintf("<span class=\"doi author\" >\n%s\n</span>", strings.Join(nameElements, ",\n"))
	return template.HTML(authorLines + "\n" + affiliationLines)
}

// JoinComma joins a slice of strings into a single string separated by commas
// (and space).  Useful for generating comma-separated lists of entries for
// templates.
func JoinComma(lst []string) string {
	return strings.Join(lst, ", ")
}

// GetGINURL returns the full URL to the configured GIN server. If it's
// configured with a non-standard port, the port number is included.
func GetGINURL(conf *Configuration) string {
	address := conf.GIN.Session.WebAddress()
	// get scheme
	schemeSepIdx := strings.Index(address, "://")
	if schemeSepIdx == -1 {
		// no scheme; return as is
		return address
	}
	// get port
	portSepIdx := strings.LastIndex(address, ":")
	if portSepIdx == -1 {
		// no port; return as is
		return address
	}
	scheme := address[:schemeSepIdx]
	port := address[portSepIdx:len(address)]
	if (scheme == "http" && port == ":80") ||
		(scheme == "https" && port == ":443") {
		// port is standard for scheme: slice it off
		address = address[0:portSepIdx]
	}
	return address
}

// FormatCitation returns the formatted citation string for a given dataset.
func FormatCitation(md *libgin.RepositoryMetadata) string {
	authors := make([]string, len(md.Creators))
	for idx, author := range md.Creators {
		namesplit := strings.SplitN(author.Name, ",", 2) // Author names are LastName, FirstName
		if len(namesplit) != 2 {
			// No comma: Bad input, mononym, or empty field.
			// Trim, add continue.
			authors[idx] = strings.TrimSpace(author.Name)
			continue
		}
		// render as LastName Initials, ...
		firstnames := strings.Fields(namesplit[1])
		var initials string
		for _, name := range firstnames {
			initials += string(name[0])
		}
		authors[idx] = fmt.Sprintf("%s %s", strings.TrimSpace(namesplit[0]), initials)
	}
	return fmt.Sprintf("%s (%d) %s. G-Node. https://doi.org/%s", strings.Join(authors, ", "), md.Year, md.Titles[0], md.Identifier.ID)
}

// FormatReferences returns the references cited by a dataset.  If the references
// are already populated in the YAMLData field they are returned as is.  If
// they are not, they are reconstructed to the YAML format from the DataCite
// metadata.  The latter can occur when loading a previously generated DataCite
// XML file instead of reading the original YAML from the repository.  If no
// references are found in either location, an empty slice is returned.
func FormatReferences(md *libgin.RepositoryMetadata) []libgin.Reference {
	if md.YAMLData != nil && len(md.YAMLData.References) != 0 {
		return md.YAMLData.References
	}

	// No references in YAML data; reconstruct from DataCite metadata if any
	// are found.

	// collect reference descriptions (descriptionType="Other")
	referenceDescriptions := make([]string, 0)
	for _, desc := range md.Descriptions {
		if desc.Type == "Other" {
			referenceDescriptions = append(referenceDescriptions, desc.Content)
		}
	}

	findDescriptionIdx := func(id string) int {
		for idx, desc := range referenceDescriptions {
			if strings.Contains(desc, id) {
				return idx
			}
		}
		return -1
	}

	splitDescriptionType := func(desc string) (string, string) {
		descParts := strings.SplitN(desc, ":", 2)
		if len(descParts) != 2 {
			return "", desc
		}

		return strings.TrimSpace(descParts[0]), strings.TrimSpace(descParts[1])
	}

	refs := make([]libgin.Reference, 0)
	// map IDs to new references for easier construction from the two sources
	// but also use the slice to maintain order
	for _, relid := range md.RelatedIdentifiers {
		if relid.RelationType == "IsVariantFormOf" || relid.RelationType == "IsIdenticalTo" {
			// IsVariantFormOf is used for the URLs.
			// IsIdenticalTo is used for the old DOI URLs.
			// Here we assume that any other type is a citation
			continue
		}
		ref := &libgin.Reference{
			ID:      fmt.Sprintf("%s:%s", relid.Type, relid.Identifier),
			RefType: relid.RelationType,
		}
		if idx := findDescriptionIdx(relid.Identifier); idx >= 0 {
			citation := referenceDescriptions[idx]
			referenceDescriptions = append(referenceDescriptions[:idx], referenceDescriptions[idx+1:]...) // remove found element
			_, citation = splitDescriptionType(citation)
			// filter out the DOI URL from the citation
			urlstr := fmt.Sprintf("(%s)", ref.GetURL())
			citation = strings.Replace(citation, urlstr, "", -1)
			ref.Citation = citation
		}
		refs = append(refs, *ref)
	}

	// Add the rest of the descriptions that didn't have an ID to match (if any)
	for _, refDesc := range referenceDescriptions {
		refType, citation := splitDescriptionType(refDesc)
		ref := libgin.Reference{
			ID:       "",
			RefType:  refType,
			Citation: citation,
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}

// FormatIssuedDate returns the issued date of the dataset in the format DD Mon.
// YYYY for adding to the preparation and landing pages.
func FormatIssuedDate(md *libgin.RepositoryMetadata) string {
	var datestr string
	for _, mddate := range md.Dates {
		// There should be only one, but we might add some other types of date
		// at some point, so best be safe.
		if mddate.Type == "Issued" {
			datestr = mddate.Value
			break
		}
	}

	date, err := time.Parse("2006-01-02", datestr)
	if err != nil {
		// This will also occur if the date isn't found in 'md' and the string
		// remains empty
		log.Printf("Failed to parse issued date: %s", datestr)
		return ""
	}
	return date.Format("02 Jan. 2006")
}

// KeywordPath returns a keyword sanitised for use in a URL path:
// Lowercase + replace / with _.
func KeywordPath(kw string) string {
	kw = strings.ToLower(kw)
	kw = strings.ReplaceAll(kw, "/", "_")
	return kw
}

// FormatAuthorList returns a comma-separated list of the author names for a
// dataset.
func FormatAuthorList(md *libgin.RepositoryMetadata) string {
	names := make([]string, len(md.Creators))
	for idx, author := range md.Creators {
		names[idx] = author.Name
	}
	authors := strings.Join(names, ", ")
	return authors
}

var templateMap = map[string]string{
	"Nav":                gdtmpl.Nav,
	"Footer":             gdtmpl.Footer,
	"RequestFailurePage": gdtmpl.RequestFailurePage,
	"RequestPage":        gdtmpl.RequestPage,
	"RequestResult":      gdtmpl.RequestResult,
	"DOIInfo":            gdtmpl.DOIInfo,
	"LandingPage":        gdtmpl.LandingPage,
	"KeywordIndex":       gdtmpl.KeywordIndex,
	"Keyword":            gdtmpl.Keyword,
}

// prepareTemplates initialises and parses a sequence of templates in the order
// they appear in the arguments.  It always adds the Nav template first and
// includes the common template functions in tmplfuncs.
func prepareTemplates(templateNames ...string) (*template.Template, error) {
	tmpl, err := template.New("Nav").Funcs(tmplfuncs).Parse(templateMap["Nav"])
	if err != nil {
		log.Printf("Could not parse the \"Nav\" template: %s", err.Error())
		return nil, err
	}
	tmpl, err = tmpl.New("Footer").Parse(templateMap["Footer"])
	if err != nil {
		log.Printf("Could not parse the \"Footer\" template: %s", err.Error())
		return nil, err
	}
	for _, tName := range templateNames {
		tContent, ok := templateMap[tName]
		if !ok {
			return nil, fmt.Errorf("Unknown template with name %q", tName)
		}
		tmpl, err = tmpl.New(tName).Parse(tContent)
		if err != nil {
			log.Printf("Could not parse the %q template: %s", tName, err.Error())
			return nil, err
		}
	}
	return tmpl, nil

}

// collectWarnings checks for non-critical missing information or issues that
// may need admin attention. These should be sent with the followup
// notification email.
func collectWarnings(job *RegistrationJob) (warnings []string) {
	// Check if any funder IDs are missing
	for _, funder := range *job.Metadata.FundingReferences {
		if funder.Identifier == nil || funder.Identifier.ID == "" {
			warnings = append(warnings, fmt.Sprintf("Couldn't find funder ID for funder %q", funder.Funder))
		}
	}

	// Check if a reference from the YAML file uses the old "Name" field instead of "Citation"
	// This shouldn't be an issue, but it can cause formatting issues
	for idx, ref := range job.Metadata.YAMLData.References {
		if ref.Name != "" {
			warnings = append(warnings, fmt.Sprintf("Reference %d uses old 'Name' field instead of 'Citation'", idx))
		}
	}

	// The 80 character limit is arbitrary, but if the abstract is very short, it's worth a check
	if absLen := len(job.Metadata.YAMLData.Description); absLen < 80 {
		warnings = append(warnings, fmt.Sprintf("Abstract may be too short: %d characters", absLen))
	}

	return
}
