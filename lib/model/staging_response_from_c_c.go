package model

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"

	"github.com/go-openapi/errors"
)

/*StagingResponseFromCC staging response from c c

swagger:model StagingResponseFromCC
*/
type StagingResponseFromCC struct {

	/* error
	 */
	Error *StagingError `json:"error,omitempty"`

	/* result
	 */
	Result interface{} `json:"result,omitempty"`
}

// Validate validates this staging response from c c
func (m *StagingResponseFromCC) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateError(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *StagingResponseFromCC) validateError(formats strfmt.Registry) error {

	if swag.IsZero(m.Error) { // not required
		return nil
	}

	if m.Error != nil {

		if err := m.Error.Validate(formats); err != nil {
			return err
		}
	}

	return nil
}
