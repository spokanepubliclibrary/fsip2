package models

import "time"

// Request represents a FOLIO request (hold)
type Request struct {
	ID                                string            `json:"id"`
	RequestType                       string            `json:"requestType"` // Hold, Recall, Page
	RequestDate                       *time.Time        `json:"requestDate"`
	PatronComments                    string            `json:"patronComments,omitempty"`
	RequesterID                       string            `json:"requesterId"`
	ProxyUserID                       string            `json:"proxyUserId,omitempty"`
	ItemID                            string            `json:"itemId"`
	InstanceID                        string            `json:"instanceId"`
	Status                            string            `json:"status"` // Open - Not yet filled, Open - Awaiting pickup, Closed - Filled, etc.
	CancellationReasonID              string            `json:"cancellationReasonId,omitempty"`
	CancelledByUserID                 string            `json:"cancelledByUserId,omitempty"`
	CancellationAdditionalInformation string            `json:"cancellationAdditionalInformation,omitempty"`
	CancelledDate                     *time.Time        `json:"cancelledDate,omitempty"`
	Position                          int               `json:"position"`
	Item                              *RequestItem      `json:"item,omitempty"`
	Requester                         *RequestRequester `json:"requester,omitempty"`
	FulfillmentPreference             string            `json:"fulfillmentPreference"` // Hold Shelf, Delivery
	DeliveryAddressTypeID             string            `json:"deliveryAddressTypeId,omitempty"`
	RequestExpirationDate             *time.Time        `json:"requestExpirationDate,omitempty"`
	HoldShelfExpirationDate           *time.Time        `json:"holdShelfExpirationDate,omitempty"`
	PickupServicePointID              string            `json:"pickupServicePointId,omitempty"`
	Metadata                          Metadata          `json:"metadata,omitempty"`
}

// RequestItem represents item information in a request
type RequestItem struct {
	Barcode    string `json:"barcode,omitempty"`
	Title      string `json:"title,omitempty"`
	CallNumber string `json:"callNumber,omitempty"`
}

// RequestRequester represents requester information in a request
type RequestRequester struct {
	FirstName  string `json:"firstName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	Barcode    string `json:"barcode,omitempty"`
}

// RequestCollection represents a collection of requests
type RequestCollection struct {
	Requests     []Request `json:"requests"`
	TotalRecords int       `json:"totalRecords"`
}

// IsOpen checks if the request is open
func (r *Request) IsOpen() bool {
	return r.Status == "Open - Not yet filled" || r.Status == "Open - Awaiting pickup" || r.Status == "Open - In transit"
}

// IsAwaitingPickup checks if the request is awaiting pickup
func (r *Request) IsAwaitingPickup() bool {
	return r.Status == "Open - Awaiting pickup"
}

// IsHold checks if the request is a hold
func (r *Request) IsHold() bool {
	return r.RequestType == "Hold" || r.RequestType == "Recall"
}
