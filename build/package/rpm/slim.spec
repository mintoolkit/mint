%global go_version 1.18.2

Name: mint
Version: 1.40.6
Release: 1%{?dist}
Summary: minT(oolkit) helps you make your containers better, smaller, and secure
License: Apache-2.0
BuildRequires: golang >= %{go_version}
URL: https://github.com/mintoolkit/mint
Source0: https://github.com/mintoolkit/mint/archive/refs/tags/%{version}.tar.gz

%define debug_package %{nil}

%prep
%autosetup

%description
minT(oolkit) helps you make your containers better, smaller, and secure

%ifarch x86_64
%define goarch amd64
%endif

%ifarch aarch64
%define goarch arm64
%endif

%ifarch arm
%define goarch arm
%endif

%global mint_version %(git describe --tags --always)
%global mint_revision %(git rev-parse HEAD)
%global mint_buildtime %(date '+%Y-%m-%d_%I:%M:%''S')
%global mint_ldflags -s -w -X github.com/mintoolkit/mint/pkg/version.appVersionTag=%{mint_version} -X github.com/mintoolkit/mint/pkg/version.appVersionRev=%{mint_revision} -X github.com/mintoolkit/mint/pkg/version.appVersionTime=%{mint_buildtime}

%build
export CGO_ENABLED=0
go generate github.com/mintoolkit/mint/pkg/appbom
mkdir dist_linux
GOOS=linux GOARCH=%{goarch} go build  -mod=vendor -trimpath -ldflags="%{mint_ldflags}" -a -tags 'netgo osusergo' -o "dist_linux/" ./cmd/mint/...
GOOS=linux GOARCH=%{goarch} go build -mod=vendor -trimpath -ldflags="%{mint_ldflags}" -a -tags 'netgo osusergo' -o "dist_linux/" ./cmd/mint-sensor/...

%install
install -d -m 755 %{buildroot}%{_bindir}
install -d -m 755 %{buildroot}%{_bindir}
install -d -m 755 %{buildroot}/usr/share/doc/mint/
install -d -m 755 %{buildroot}/usr/share/licenses/mint/
install -m 755 dist_linux/%{name} %{buildroot}%{_bindir}
install -m 755 dist_linux/%{name}-sensor %{buildroot}%{_bindir}
install -m 644 README.md %{buildroot}/usr/share/doc/mint/README.md
install -m 644 LICENSE %{buildroot}/usr/share/licenses/mint/LICENSE

%post
%{__ln_s} -f %{_bindir}/%{name} %{_bindir}/docker-slim
chmod a+x %{_bindir}/%{name}
chmod a+x %{_bindir}/%{name}-sensor

%files 
%{_bindir}/%{name}
%{_bindir}/%{name}-sensor
%doc /usr/share/doc/mint/README.md
%license /usr/share/licenses/mint/LICENSE
