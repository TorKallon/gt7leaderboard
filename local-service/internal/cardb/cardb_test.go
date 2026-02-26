package cardb

import (
	"strings"
	"testing"
)

const testCSV = `carid,manufacturer,name,group,CH,CM,CS,SH,SM,SS,RH,RM,RS,IM,W,D
1,Toyota,GR Supra,N,550.0,560.0,570.0,580.0,590.0,600.0,610.0,620.0,630.0,640.0,1500,4.5
2,Honda,NSX,3,700.0,710.0,720.0,730.0,740.0,750.0,760.0,770.0,780.0,790.0,1200,4.2
3,Nissan,GT-R,1,850.0,860.0,870.0,880.0,890.0,900.0,910.0,920.0,930.0,940.0,1600,5.0
4,BMW,M3,B,400.0,410.0,420.0,430.0,440.0,450.0,460.0,470.0,480.0,490.0,1400,4.8
5,Porsche,911 RSR,2,650.0,660.0,670.0,680.0,690.0,700.0,710.0,720.0,730.0,740.0,1300,4.1
6,Ferrari,F40,4,450.0,460.0,470.0,480.0,490.0,500.0,510.0,520.0,530.0,540.0,1350,4.6
7,Mazda,RX-7,X,250.0,260.0,270.0,280.0,290.0,300.0,310.0,320.0,330.0,340.0,1100,3.9
`

func TestLoadFromReader(t *testing.T) {
	db, err := LoadFromReader(strings.NewReader(testCSV))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	if db.Count() != 7 {
		t.Errorf("Count() = %d, want 7", db.Count())
	}
}

func TestLookup_Existing(t *testing.T) {
	db, err := LoadFromReader(strings.NewReader(testCSV))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	car, ok := db.Lookup(1)
	if !ok {
		t.Fatal("Lookup(1) returned false, want true")
	}
	if car.Name != "GR Supra" {
		t.Errorf("Name = %q, want %q", car.Name, "GR Supra")
	}
	if car.Manufacturer != "Toyota" {
		t.Errorf("Manufacturer = %q, want %q", car.Manufacturer, "Toyota")
	}
	if car.PPStock != 550.0 {
		t.Errorf("PPStock = %f, want 550.0", car.PPStock)
	}
}

func TestLookup_Missing(t *testing.T) {
	db, err := LoadFromReader(strings.NewReader(testCSV))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	_, ok := db.Lookup(9999)
	if ok {
		t.Error("Lookup(9999) returned true, want false")
	}
}

func TestCategoryNormalization(t *testing.T) {
	db, err := LoadFromReader(strings.NewReader(testCSV))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	tests := []struct {
		carID    int
		wantCat  string
	}{
		{1, "N"},
		{2, "Gr.3"},
		{3, "Gr.1"},
		{4, "Gr.B"},
		{5, "Gr.2"},
		{6, "Gr.4"},
		{7, "Gr.X"},
	}

	for _, tc := range tests {
		car, ok := db.Lookup(tc.carID)
		if !ok {
			t.Errorf("Lookup(%d) not found", tc.carID)
			continue
		}
		if car.Category != tc.wantCat {
			t.Errorf("car %d Category = %q, want %q", tc.carID, car.Category, tc.wantCat)
		}
	}
}

func TestAll(t *testing.T) {
	db, err := LoadFromReader(strings.NewReader(testCSV))
	if err != nil {
		t.Fatalf("LoadFromReader failed: %v", err)
	}

	all := db.All()
	if len(all) != 7 {
		t.Errorf("All() returned %d cars, want 7", len(all))
	}
}

func TestPPSubBand(t *testing.T) {
	tests := []struct {
		pp   float64
		want string
	}{
		{100, "N100-300"},
		{250, "N100-300"},
		{299.9, "N100-300"},
		{300, "N300-500"},
		{400, "N300-500"},
		{499.9, "N300-500"},
		{500, "N500-700"},
		{600, "N500-700"},
		{699.9, "N500-700"},
		{700, "N700+"},
		{800, "N700+"},
		{1000, "N700+"},
	}

	for _, tc := range tests {
		got := PPSubBand(tc.pp)
		if got != tc.want {
			t.Errorf("PPSubBand(%f) = %q, want %q", tc.pp, got, tc.want)
		}
	}
}
