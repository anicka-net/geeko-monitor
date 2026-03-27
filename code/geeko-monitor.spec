#
# spec file for package geeko-monitor
#
# Copyright (c) 2026 SUSE LLC
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License).

Name:           geeko-monitor
Version:        0.2.0
Release:        0
Summary:        System health monitoring dashboard
License:        MIT
URL:            https://github.com/suse/geeko-monitor
Source0:        %{name}-%{version}.tar.gz
BuildRequires:  golang(API) >= 1.21
Requires(pre):  shadow

%description
Geeko Monitor is a system health monitoring dashboard that runs as a
systemd service and serves a web UI with live-updating gauges and charts
for CPU, memory, disk, network, and temperature metrics.

%prep
%setup -q

%build
go build -o %{name} .

%install
install -D -m 0755 %{name} %{buildroot}%{_bindir}/%{name}
install -D -m 0644 %{name}.service %{buildroot}%{_unitdir}/%{name}.service
install -d -m 0755 %{buildroot}%{_sysconfdir}/%{name}

%pre
getent group geeko-monitor >/dev/null || groupadd -r geeko-monitor
getent passwd geeko-monitor >/dev/null || useradd -r -g geeko-monitor \
    -d /nonexistent -s /sbin/nologin \
    -c "Geeko Monitor service user" geeko-monitor

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%postun
%systemd_postun_with_restart %{name}.service

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%{_unitdir}/%{name}.service
%dir %{_sysconfdir}/%{name}
