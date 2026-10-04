/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package ezbk

// Wire types of the ezBookkeeping API. IDs are int64 encoded as JSON strings.

type userProfile struct {
	Username        string `json:"username"`
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	DefaultCurrency string `json:"defaultCurrency"`
	DefaultAccount  string `json:"defaultAccountId"`
	FirstDayOfWeek  int    `json:"firstDayOfWeek"`
}

type accountInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	ParentID    string         `json:"parentId"`
	Category    int            `json:"category"`
	Type        int            `json:"type"`
	Currency    string         `json:"currency"`
	Balance     string         `json:"balance"`
	Comment     string         `json:"comment"`
	IsAsset     bool           `json:"isAsset"`
	IsLiability bool           `json:"isLiability"`
	Hidden      bool           `json:"hidden"`
	SubAccounts []*accountInfo `json:"subAccounts"`
}

type accountCreateRequest struct {
	Name     string `json:"name"`
	Category int    `json:"category"`
	Type     int    `json:"type"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
	Currency string `json:"currency"`
	Comment  string `json:"comment,omitempty"`
}

type categoryInfo struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	ParentID      string          `json:"parentId"`
	Type          int             `json:"type"`
	Comment       string          `json:"comment"`
	Hidden        bool            `json:"hidden"`
	SubCategories []*categoryInfo `json:"subCategories"`
}

type categoryCreateRequest struct {
	Name     string `json:"name"`
	Type     int    `json:"type"`
	ParentID string `json:"parentId"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
	Comment  string `json:"comment,omitempty"`
}

type tagInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	GroupID string `json:"groupId"`
	Hidden  bool   `json:"hidden"`
}

type tagCreateRequest struct {
	GroupID string `json:"groupId"`
	Name    string `json:"name"`
}

type transactionInfo struct {
	ID                   string   `json:"id"`
	Type                 int      `json:"type"`
	CategoryID           string   `json:"categoryId"`
	Time                 int64    `json:"time"`
	UtcOffset            int      `json:"utcOffset"`
	SourceAccountID      string   `json:"sourceAccountId"`
	DestinationAccountID string   `json:"destinationAccountId"`
	SourceAmount         int64    `json:"sourceAmount"`
	DestinationAmount    *int64   `json:"destinationAmount"`
	TagIDs               []string `json:"tagIds"`
	Comment              string   `json:"comment"`
	Editable             bool     `json:"editable"`
}

type templateInfo struct {
	transactionInfo
	Name         string `json:"name"`
	TemplateType int    `json:"templateType"`
	Hidden       bool   `json:"hidden"`
}

type transactionWriteRequest struct {
	ID                   string   `json:"id,omitempty"`
	Type                 int      `json:"type"`
	CategoryID           string   `json:"categoryId"`
	Time                 int64    `json:"time"`
	UtcOffset            int      `json:"utcOffset"`
	SourceAccountID      string   `json:"sourceAccountId"`
	DestinationAccountID string   `json:"destinationAccountId"`
	SourceAmount         int64    `json:"sourceAmount"`
	DestinationAmount    int64    `json:"destinationAmount"`
	TagIDs               []string `json:"tagIds"`
	PictureIDs           []string `json:"pictureIds"`
	Comment              string   `json:"comment"`
}

type idRequest struct {
	ID string `json:"id"`
}
