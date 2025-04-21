%global go_version 1.23

Name:           mkbrr
Version:        1.9.0
Release:        1%{?dist}
Summary:        Tool for creating, inspecting and modifying .torrent files

License:        GPLv2
URL:            https://github.com/autobrr/mkbrr
Source0:        %{url}/archive/v%{version}/%{name}-%{version}.tar.gz

BuildRequires:  golang >= %{go_version}

%description
mkbrr is a tool for creating, inspecting and modifying .torrent files.

%prep
%autosetup -p1

%build
export CGO_ENABLED=0
BUILDTIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
go build -v -ldflags="-s -w -X main.version=%{version} -X main.buildTime=${BUILDTIME}" -o %{name} ./main.go

%install
install -D -m 755 %{name} %{buildroot}%{_bindir}/%{name}
install -D -m 644 LICENSE %{buildroot}%{_licensedir}/%{name}/LICENSE

%files
%{_bindir}/%{name}
%license %{_licensedir}/%{name}/LICENSE

%changelog
* Mon Apr 21 2025 soup <soup@mkbrr.com> - 1.9.0
- Initial package
