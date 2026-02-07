# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Features added but not yet released

### Changed
- Changes in existing functionality

### Fixed
- Bug fixes

### Deprecated
- Features to be removed in future versions

### Removed
- Removed features

### Security
- Security vulnerability fixes

## [0.1.0] - 2024-01-11

### Added
- Initial release of HydraDNS
- DNS forwarding with caching
- Custom DNS support (A, AAAA, CNAME records)
- Rate limiting (3-tier token bucket)
- Domain filtering with remote blocklists
- EDNS(0) support
- DNSSEC-aware forwarding
- REST API with Swagger documentation
- Systemd service installation script
- Comprehensive configuration options
- Buffer pooling for performance
- Singleflight deduplication
- TCP and UDP support with automatic fallback
