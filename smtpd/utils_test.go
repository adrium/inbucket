package smtpd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMailboxName(t *testing.T) {
	var validTable = []struct {
		input  string
		expect string
	}{
		{"mailbox", "mailbox"},
		{"user123", "user123"},
		{"MailBOX", "mailbox"},
		{"First.Last", "first.last"},
		{"user+label", "user"},
		{"chars!#$%", "chars!#$%"},
		{"chars&'*-", "chars&'*-"},
		{"chars=/?^", "chars=/?^"},
		{"chars_`.{", "chars_`.{"},
		{"chars|}~", "chars|}~"},
	}

	for _, tt := range validTable {
		if result, err := ParseMailboxName(tt.input); err != nil {
			t.Errorf("Error while parsing %q: %v", tt.input, err)
		} else {
			if result != tt.expect {
				t.Errorf("Parsing %q, expected %q, got %q", tt.input, tt.expect, result)
			}
		}
	}

	var invalidTable = []struct {
		input, msg string
	}{
		{"", "Empty mailbox name is not permitted"},
		{"user@host", "@ symbol not permitted"},
		{"first last", "Space not permitted"},
		{"first\"last", "Double quote not permitted"},
		{"first\nlast", "Control chars not permitted"},
	}

	for _, tt := range invalidTable {
		if _, err := ParseMailboxName(tt.input); err == nil {
			t.Errorf("Didn't get an error while parsing %q: %v", tt.input, tt.msg)
		}
	}
}

func TestHashMailboxName(t *testing.T) {
	assert.Equal(t, HashMailboxName("mail"), "1d6e1cf70ec6f9ab28d3ea4b27a49a77654d370e")
}

func TestValidateDomain(t *testing.T) {
	assert.False(t, ValidateDomainPart(strings.Repeat("a", 256)),
		"Max domain length is 255")
	assert.False(t, ValidateDomainPart(strings.Repeat("a", 64)+".com"),
		"Max label length is 63")
	assert.True(t, ValidateDomainPart(strings.Repeat("a", 63)+".com"),
		"Should allow 63 char label")

	var testTable = []struct {
		input  string
		expect bool
		msg    string
	}{
		{"", false, "Empty domain is not valid"},
		{"hostname", true, "Just a hostname is valid"},
		{"github.com", true, "Two labels should be just fine"},
		{"my-domain.com", true, "Hyphen is allowed mid-label"},
		{"_domainkey.foo.com", true, "Underscores are allowed"},
		{"bar.com.", true, "Must be able to end with a dot"},
		{"ABC.6DBS.com", true, "Mixed case is OK"},
		{"mail.123.com", true, "Number only label valid"},
		{"123.com", true, "Number only label valid"},
		{"google..com", false, "Double dot not valid"},
		{".foo.com", false, "Cannot start with a dot"},
		{"google\r.com", false, "Special chars not allowed"},
		{"foo.-bar.com", false, "Label cannot start with hyphen"},
		{"foo-.bar.com", false, "Label cannot end with hyphen"},
	}

	for _, tt := range testTable {
		if ValidateDomainPart(tt.input) != tt.expect {
			t.Errorf("Expected %v for %q: %s", tt.expect, tt.input, tt.msg)
		}
	}
}

func TestValidateLocal(t *testing.T) {
	var testTable = []struct {
		input  string
		expect bool
		msg    string
	}{
		{"", false, "Empty local is not valid"},
		{"a", true, "Single letter should be fine"},
		{strings.Repeat("a", 128), true, "Valid up to 128 characters"},
		{strings.Repeat("a", 129), false, "Only valid up to 128 characters"},
		{"FirstLast", true, "Mixed case permitted"},
		{"user123", true, "Numbers permitted"},
		{"a!#$%&'*+-/=?^_`{|}~", true, "Any of !#$%&'*+-/=?^_`{|}~ are permitted"},
		{"first.last", true, "Embedded period is permitted"},
		{"first..last", false, "Sequence of periods is not allowed"},
		{".user", false, "Cannot lead with a period"},
		{"user.", false, "Cannot end with a period"},
		{"james@mail", false, "Unquoted @ not permitted"},
		{"first last", false, "Unquoted space not permitted"},
		{"tricky\\. ", false, "Unquoted space not permitted"},
		{"no,commas", false, "Unquoted comma not allowed"},
		{"t[es]t", false, "Unquoted square brackets not allowed"},
		{"james\\", false, "Cannot end with backslash quote"},
		{"james\\@mail", true, "Quoted @ permitted"},
		{"quoted\\ space", true, "Quoted space permitted"},
		{"no\\,commas", true, "Quoted comma is OK"},
		{"t\\[es\\]t", true, "Quoted brackets are OK"},
		{"user\\name", true, "Should be able to quote a-z"},
		{"USER\\NAME", true, "Should be able to quote A-Z"},
		{"user\\1", true, "Should be able to quote a digit"},
		{"one\\$\\|", true, "Should be able to quote plain specials"},
		{"return\\\r", true, "Should be able to quote ASCII control chars"},
		{"high\\\x80", false, "Should not accept > 7-bit quoted chars"},
		{"quote\\\"", true, "Quoted double quote is permitted"},
		{"\"james\"", true, "Quoted a-z is permitted"},
		{"\"first last\"", true, "Quoted space is permitted"},
		{"\"quoted@sign\"", true, "Quoted @ is allowed"},
		{"\"qp\\\"quote\"", true, "Quoted quote within quoted string is OK"},
		{"\"unterminated", false, "Quoted string must be terminated"},
		{"\"unterminated\\\"", false, "Quoted string must be terminated"},
		{"embed\"quote\"string", false, "Embedded quoted string is illegal"},

		{"user+mailbox", true, "RFC3696 test case should be valid"},
		{"customer/department=shipping", true, "RFC3696 test case should be valid"},
		{"$A12345", true, "RFC3696 test case should be valid"},
		{"!def!xyz%abc", true, "RFC3696 test case should be valid"},
		{"_somename", true, "RFC3696 test case should be valid"},
	}

	for _, tt := range testTable {
		_, _, err := ParseEmailAddress(tt.input + "@domain.com")
		if (err != nil) == tt.expect {
			if err != nil {
				t.Logf("Got error: %s", err)
			}
			t.Errorf("Expected %v for %q: %s", tt.expect, tt.input, tt.msg)
		}
	}
}

func TestParseEmailAddress(t *testing.T) {
	// Test some good email addresses
	var testTable = []struct {
		input, local, domain string
	}{
		{"root@localhost", "root", "localhost"},
		{"FirstLast@domain.local", "FirstLast", "domain.local"},
		{"route66@prodigy.net", "route66", "prodigy.net"},
		{"lorbit!user@uucp", "lorbit!user", "uucp"},
		{"user+spam@gmail.com", "user+spam", "gmail.com"},
		{"first.last@domain.local", "first.last", "domain.local"},
		{"first\\ last@_key.domain.com", "first last", "_key.domain.com"},
		{"first\\\"last@a.b.c", "first\"last", "a.b.c"},
		{"user\\@internal@myhost.ca", "user@internal", "myhost.ca"},
		{"\"first last@evil\"@top-secret.gov", "first last@evil", "top-secret.gov"},
		{"\"line\nfeed\"@linenoise.co.uk", "line\nfeed", "linenoise.co.uk"},
		{"user+mailbox@host", "user+mailbox", "host"},
		{"customer/department=shipping@host", "customer/department=shipping", "host"},
		{"$A12345@host", "$A12345", "host"},
		{"!def!xyz%abc@host", "!def!xyz%abc", "host"},
		{"_somename@host", "_somename", "host"},
	}

	for _, tt := range testTable {
		local, domain, err := ParseEmailAddress(tt.input)
		if err != nil {
			t.Errorf("Error when parsing %q: %s", tt.input, err)
		} else {
			if tt.local != local {
				t.Errorf("When parsing %q, expected local %q, got %q instead",
					tt.input, tt.local, local)
			}
			if tt.domain != domain {
				t.Errorf("When parsing %q, expected domain %q, got %q instead",
					tt.input, tt.domain, domain)
			}
		}
	}

	// Check that validations fail correctly
	var badTable = []struct {
		input, msg string
	}{
		{"", "Empty address not permitted"},
		{"user", "Missing domain part"},
		{"@host", "Missing local part"},
		{"user\\@host", "Missing domain part"},
		{"\"user@host\"", "Missing domain part"},
		{"\"user@host", "Unterminated quoted string"},
		{"first last@host", "Unquoted space"},
		{"user@bad!domain", "Invalid domain"},
		{".user@host", "Can't lead with a ."},
		{"user.@host", "Can't end local with a dot"},
		{"user@bad domain", "No spaces in domain permitted"},
	}

	for _, tt := range badTable {
		if _, _, err := ParseEmailAddress(tt.input); err == nil {
			t.Errorf("Did not get expected error when parsing %q: %s", tt.input, tt.msg)
		}
	}
}
