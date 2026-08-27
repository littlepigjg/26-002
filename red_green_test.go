package ubaas_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.New(os.Stdout, logger.LevelError, "")
	memStore := store.NewMemoryStore(log)
	defer memStore.Close()

	es := service.NewEventService(memStore, cfg, log)
	ctx := context.Background()

	req1 := &model.EventCreateRequest{
		UserID:     "",
		Type:       model.EventPageView,
		PageURL:    "/home",
		DeviceType: model.DeviceMobile,
		OS:         "Android",
		Browser:    "Chrome",
		Country:    "US",
	}

	_, err := es.CreateEvent(ctx, req1)
	if err != nil {
		t.Fatalf("First CreateEvent failed: %v", err)
	}

	req2 := &model.EventCreateRequest{
		UserID:  "",
		Type:    model.EventClick,
		PageURL: "/button",
	}

	_, err = es.CreateEvent(ctx, req2)
	if err != nil {
		t.Fatalf("Second CreateEvent failed: %v", err)
	}

	ud, err := es.GetUserDimension(ctx, "anonymous-user")
	if err != nil {
		t.Fatalf("GetUserDimension failed: %v", err)
	}

	if ud == nil {
		t.Fatal("UserDimension is nil, expected to exist")
	}

	hasPollution := false
	if ud.DeviceType == "" {
		hasPollution = true
		t.Log("DeviceType is empty - state pollution detected")
	}
	if ud.OS == "" {
		hasPollution = true
		t.Log("OS is empty - state pollution detected")
	}
	if ud.Browser == "" {
		hasPollution = true
		t.Log("Browser is empty - state pollution detected")
	}
	if ud.Country == "" {
		hasPollution = true
		t.Log("Country is empty - state pollution detected")
	}

	if hasPollution {
		fmt.Println("RED（红灯，缺陷未修复）- UserDimension fields polluted with empty values")
		t.Error("RED: UserDimension fields were polluted - empty values overwrote previously valid dimension data")
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）- UserDimension fields preserved correctly")
	}
}
