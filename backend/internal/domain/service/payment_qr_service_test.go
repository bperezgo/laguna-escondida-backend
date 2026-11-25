package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	qrPublicKeyFixture = "pub_test_laguna"
	qrIntegrityFixture = "test_integrity_laguna"
	qrSVG              = `<svg xmlns="http://www.w3.org/2000/svg"/>`
)

type qrFixture struct {
	qrs      *mocks.MockPaymentQRRepository
	provider *mocks.MockPaymentQRProvider
	config   *mocks.MockPaymentLinkConfig
	now      time.Time
	service  *PaymentQRService
}

func qrCredentials() dto.WompiCredentials {
	return dto.WompiCredentials{PrivateKey: linkPrivateKey, EventsSecret: "test_events_laguna", PublicKey: qrPublicKeyFixture, IntegritySecret: qrIntegrityFixture}
}

func newQRFixture(t *testing.T, creds dto.WompiCredentials, wait time.Duration) *qrFixture {
	t.Helper()
	f := &qrFixture{
		qrs: mocks.NewMockPaymentQRRepository(t), provider: mocks.NewMockPaymentQRProvider(t), config: mocks.NewMockPaymentLinkConfig(t),
		now: time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC),
	}
	f.config.EXPECT().For(mock.Anything, crmOrgID).Return(creds, true, nil).Maybe()
	f.service = NewPaymentQRService(f.qrs, f.provider, f.config, crmOrganizations(t), "https://api.example.com/", wait,
		slog.New(slog.DiscardHandler), func() time.Time { return f.now })
	f.service.pollEvery = time.Millisecond
	f.service.newToken = func() (string, error) { return "tok-1", nil }
	return f
}

func merchantFixture() dto.WompiMerchantTerms {
	return dto.WompiMerchantTerms{
		TermsURL: "https://wompi.com/terms.pdf", PersonalDataURL: "https://wompi.com/data.pdf", Email: "restaurante@example.com",
		AcceptanceToken: "acc-1", PersonalAuthToken: "per-1",
	}
}

func (f *qrFixture) expectCreation(txID string) {
	f.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(merchantFixture(), nil).Once()
	f.provider.EXPECT().CreateQRTransaction(mock.Anything, linkPrivateKey, mock.Anything).Return(txID, nil).Once()
}

var qrExpiry = time.Date(2026, 11, 24, 17, 30, 0, 0, time.UTC)

func TestPaymentQRService_CreateForOrder_AQRForTheTotal(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(merchantFixture(), nil).Once()
	f.provider.EXPECT().CreateQRTransaction(mock.Anything, linkPrivateKey, mock.MatchedBy(func(req dto.QRTransactionRequest) bool {
		sum := sha256.Sum256([]byte(req.Reference + "4500000" + "COP" + "2026-11-24T17:30:00.000Z" + qrIntegrityFixture))
		return req.AmountInCents == 4500000 && req.Currency == "COP" && req.AcceptanceToken == "acc-1" && req.PersonalAuthToken == "per-1" &&
			req.CustomerEmail == "restaurante@example.com" && req.Signature == hex.EncodeToString(sum[:]) &&
			req.ExpiresAt != nil && req.ExpiresAt.Equal(qrExpiry) && len(req.Reference) > 10
	})).Return("12022516-1-1", nil).Once()
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "12022516-1-1").Return(qrSVG, true, nil).Once()
	f.qrs.EXPECT().Create(mock.Anything, mock.MatchedBy(func(qr *dto.PaymentQRTransaction) bool {
		return qr.OrderID == linkOrderID && qr.WompiTransactionID == "12022516-1-1" && qr.SVG == qrSVG && qr.URLToken == "tok-1" &&
			qr.Status == dto.PaymentQRActive && qr.AmountInCents == 4500000
	})).Return(nil).Once()

	url := f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry)

	require.NotNil(t, url)
	assert.Equal(t, "https://api.example.com/api/payments/qr/tok-1", *url)
}

func TestPaymentQRService_CreateForOrder_EachQRHasItsOwnReference(t *testing.T) {
	assert.NotEqual(t, qrReference(linkOrderID), qrReference(linkOrderID))
	assert.Contains(t, qrReference(linkOrderID), "qr-")
}

func TestPaymentQRService_CreateForOrder_NoQRWithoutTheTerms(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)

	url := f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), false, qrExpiry)

	assert.Nil(t, url, "nothing is asked of Wompi")
}

func TestPaymentQRService_CreateForOrder_NoQRWithoutTheCredentials(t *testing.T) {
	creds := qrCredentials()
	creds.PublicKey, creds.IntegritySecret = "", ""
	f := newQRFixture(t, creds, time.Second)

	url := f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry)

	assert.Nil(t, url, "an integration saved before the QR keys keeps the link only")
}

func TestPaymentQRService_CreateForOrder_WompiRefusesTheQR(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(merchantFixture(), nil).Once()
	f.provider.EXPECT().CreateQRTransaction(mock.Anything, linkPrivateKey, mock.Anything).Return("", errors.New("422")).Once()

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry))
}

func TestPaymentQRService_CreateForOrder_TermsUnavailable(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(dto.WompiMerchantTerms{}, errors.New("503")).Once()

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry))
}

func TestPaymentQRService_CreateForOrder_TheQRArrivesLate(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), 5*time.Millisecond)
	f.expectCreation("tx-late")
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-late").Return("", false, nil)
	f.qrs.EXPECT().Create(mock.Anything, mock.MatchedBy(func(qr *dto.PaymentQRTransaction) bool { return qr.SVG == "" })).Return(nil).Once()

	url := f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry)

	assert.Nil(t, url, "the order is created with its link only")
}

func TestPaymentQRService_CreateForOrder_AnImageReadFailureCountsAsNotArrived(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), 3*time.Millisecond)
	f.expectCreation("tx-1")
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-1").Return("", false, errors.New("timeout"))
	f.qrs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry))
}

func TestPaymentQRService_CreateForOrder_PollsUntilTheImageArrives(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.expectCreation("tx-1")
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-1").Return("", false, nil).Once()
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-1").Return(qrSVG, true, nil).Once()
	f.qrs.EXPECT().Create(mock.Anything, mock.MatchedBy(func(qr *dto.PaymentQRTransaction) bool { return qr.SVG == qrSVG })).Return(nil).Once()

	assert.NotNil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry))
}

func TestPaymentQRService_CreateForOrder_NotStoredMeansNobodyReceivedIt(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.expectCreation("tx-1")
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-1").Return(qrSVG, true, nil).Once()
	f.qrs.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", f.now), true, qrExpiry))
}

func TestPaymentQRService_ReplaceForOrder_ARetiredQRGetsANewOne(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(&dto.PaymentQRTransaction{ID: "qr-A", OrderID: linkOrderID}, true, nil).Once()
	f.qrs.EXPECT().RetireByOrder(mock.Anything, linkOrderID).Return(nil).Once()
	f.expectCreation("tx-B")
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-B").Return(qrSVG, true, nil).Once()
	f.qrs.EXPECT().Create(mock.Anything, mock.MatchedBy(func(qr *dto.PaymentQRTransaction) bool { return qr.AmountInCents == 9000000 })).Return(nil).Once()

	url := f.service.ReplaceForOrder(crmContext(), linkOrder("90000.0000", f.now), qrExpiry)

	assert.NotNil(t, url)
}

func TestPaymentQRService_ReplaceForOrder_AnOrderWithoutAQRGetsNone(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(nil, false, nil).Once()

	assert.Nil(t, f.service.ReplaceForOrder(crmContext(), linkOrder("90000.0000", f.now), qrExpiry))
}

func TestPaymentQRService_ReplaceForOrder_PastTheExpiryNoNewQR(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(&dto.PaymentQRTransaction{ID: "qr-A"}, true, nil).Once()
	f.qrs.EXPECT().RetireByOrder(mock.Anything, linkOrderID).Return(nil).Once()

	assert.Nil(t, f.service.ReplaceForOrder(crmContext(), linkOrder("90000.0000", f.now), f.now.Add(-time.Minute)))
}

func TestPaymentQRService_ReplaceForOrder_ReadFailure(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(nil, false, errors.New("db down")).Once()

	assert.Nil(t, f.service.ReplaceForOrder(crmContext(), linkOrder("90000.0000", f.now), qrExpiry))
}

func TestPaymentQRService_Retire(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().RetireByOrder(mock.Anything, linkOrderID).Return(nil).Once()
	f.service.Retire(crmContext(), linkOrderID)

	f.qrs.EXPECT().RetireByOrder(mock.Anything, linkOrderID).Return(errors.New("db down")).Once()
	f.service.Retire(crmContext(), linkOrderID)
}

func TestPaymentQRService_ActiveURL(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(&dto.PaymentQRTransaction{ID: "qr-A", SVG: qrSVG, URLToken: "tok-A"}, true, nil).Once()

	url := f.service.ActiveURL(crmContext(), linkOrderID)

	require.NotNil(t, url)
	assert.Equal(t, "https://api.example.com/api/payments/qr/tok-A", *url)
}

func TestPaymentQRService_ActiveURL_NoneOrReadFailure(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(nil, false, nil).Once()
	assert.Nil(t, f.service.ActiveURL(crmContext(), linkOrderID))
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(nil, false, errors.New("db down")).Once()
	assert.Nil(t, f.service.ActiveURL(crmContext(), linkOrderID))
}

func TestPaymentQRService_ActiveURL_ALateQRIsFetchedOnce(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	late := &dto.PaymentQRTransaction{ID: "qr-A", WompiTransactionID: "tx-A", URLToken: "tok-A"}
	f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(late, true, nil).Once()
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-A").Return(qrSVG, true, nil).Once()
	f.qrs.EXPECT().SetSVG(mock.Anything, "qr-A", qrSVG).Return(nil).Once()

	url := f.service.ActiveURL(crmContext(), linkOrderID)

	require.NotNil(t, url)
	assert.Contains(t, *url, "tok-A")
}

func TestPaymentQRService_ActiveURL_ALateQRStillMissing(t *testing.T) {
	for name, image := range map[string]func(f *qrFixture){
		"not ready": func(f *qrFixture) {
			f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-A").Return("", false, nil).Once()
		},
		"read failure": func(f *qrFixture) {
			f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-A").Return("", false, errors.New("503")).Once()
		},
		"not stored": func(f *qrFixture) {
			f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, "tx-A").Return(qrSVG, true, nil).Once()
			f.qrs.EXPECT().SetSVG(mock.Anything, "qr-A", qrSVG).Return(errors.New("db down")).Once()
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newQRFixture(t, qrCredentials(), time.Second)
			f.qrs.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).
				Return(&dto.PaymentQRTransaction{ID: "qr-A", WompiTransactionID: "tx-A", URLToken: "tok-A"}, true, nil).Once()
			image(f)

			assert.Nil(t, f.service.ActiveURL(crmContext(), linkOrderID))
		})
	}
}

func TestPaymentQRService_ServeSVG(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.qrs.EXPECT().FindActiveByToken(mock.Anything, "tok-A").Return(&dto.PaymentQRTransaction{SVG: qrSVG}, true, nil).Once()
	f.qrs.EXPECT().FindActiveByToken(mock.Anything, "tok-B").Return(nil, false, nil).Once()
	f.qrs.EXPECT().FindActiveByToken(mock.Anything, "tok-C").Return(&dto.PaymentQRTransaction{}, true, nil).Once()
	f.qrs.EXPECT().FindActiveByToken(mock.Anything, "tok-D").Return(nil, false, errors.New("db down")).Once()

	svg, found, err := f.service.ServeSVG(crmContext(), "tok-A")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, qrSVG, svg)
	_, found, _ = f.service.ServeSVG(crmContext(), "tok-B")
	assert.False(t, found, "a retired or unknown token")
	_, found, _ = f.service.ServeSVG(crmContext(), "tok-C")
	assert.False(t, found, "a QR that has not arrived")
	_, _, err = f.service.ServeSVG(crmContext(), "tok-D")
	require.Error(t, err)
}

func TestPaymentQRService_Terms_AreCachedForAnHour(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(merchantFixture(), nil).Twice()

	first := f.service.Terms(crmContext())
	second := f.service.Terms(crmContext())
	f.now = f.now.Add(59 * time.Minute)
	third := f.service.Terms(crmContext())
	f.now = f.now.Add(2 * time.Minute)
	fourth := f.service.Terms(crmContext())

	require.NotNil(t, first)
	assert.Equal(t, dto.PaymentTerms{TermsURL: "https://wompi.com/terms.pdf", PersonalDataURL: "https://wompi.com/data.pdf"}, *first)
	assert.Equal(t, first, second)
	assert.Equal(t, first, third, "still cached 59 minutes later")
	assert.NotNil(t, fourth, "read again after the hour")
}

func TestPaymentQRService_Terms_NoneWhenNoQRCanBeOffered(t *testing.T) {
	creds := qrCredentials()
	creds.PublicKey = ""
	f := newQRFixture(t, creds, time.Second)

	assert.Nil(t, f.service.Terms(crmContext()))
}

func TestPaymentQRService_Terms_WompiDoesNotAnswer(t *testing.T) {
	f := newQRFixture(t, qrCredentials(), time.Second)
	f.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(dto.WompiMerchantTerms{}, errors.New("503")).Once()

	assert.Nil(t, f.service.Terms(crmContext()))
}
