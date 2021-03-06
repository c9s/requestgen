package requestgen

import "testing"

func TestParseTypeSelector(t *testing.T) {
	type args struct {
		main string
	}
	tests := []struct {
		name    string
		args    args
		wantErr  bool
		wantSpec TypeSelector
	}{
		{
			name: "full-qualified",
			args: args{
				main: `"github.com/c9s/requestgen".APIClient`,
			},
			wantErr: false,
			wantSpec: TypeSelector{
				Package: "github.com/c9s/requestgen",
				Member:  "APIClient",
			},
		},
		{
			name: "cwd",
			args: args{
				main: `".".APIClient`,
			},
			wantErr: false,
			wantSpec: TypeSelector{
				Package: "github.com/c9s/requestgen",
				Member:  "APIClient",
			},
		},
		{
			name: "slice",
			args: args{
				main: `[]APIClient`,
			},
			wantErr: false,
			wantSpec: TypeSelector{
				Package: "github.com/c9s/requestgen",
				Member:  "APIClient",
				IsSlice: true,
			},
		},
		{
			name: "ident",
			args: args{
				main: `APIClient`,
			},
			wantErr: false,
			wantSpec: TypeSelector{
				Package: "github.com/c9s/requestgen",
				Member:  "APIClient",
			},
		},
		{
			name: "single dot",
			args: args{
				main: `.APIClient`,
			},
			wantErr: false,
			wantSpec: TypeSelector{
				Package: "github.com/c9s/requestgen",
				Member:  "APIClient",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseTypeSelector(tt.args.main)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTypeSelector() error = %v, wantErr %v", err, tt.wantErr)
			} else if spec != nil && *spec != tt.wantSpec {
				t.Errorf("ParseTypeSelector() TypeSelector = %+v, wantSpec %+v", spec, tt.wantSpec)
			}
		})
	}
}
