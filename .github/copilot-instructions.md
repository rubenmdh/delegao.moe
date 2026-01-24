# AI Coding Agent Instructions for delegao.moe

## Project Overview
Laravel 12 web application with Vite frontend build, Tailwind CSS styling, and SQLite database. Minimal starter configuration with profile management and authentication scaffolding.

## Development Workflow

### Setup & Initialization
```bash
composer run setup  # One-command setup: installs deps, generates key, runs migrations, builds frontend
```

### Running the Application
```bash
composer run dev  # Starts 4 concurrent processes:
                  # - php artisan serve (port 8000)
                  # - php artisan queue:listen
                  # - php artisan pail (log viewer)
                  # - npm run dev (Vite HMR)
```

### Testing
```bash
composer run test  # Clears config cache, runs PHPUnit (Unit + Feature suites)
npm run build      # Production frontend build
```

## Architecture Patterns

### File Structure
- **app/** - PHP application code (Controllers, Models, Requests)
- **resources/views/** - Blade templates
- **resources/css|js/** - Frontend assets (Tailwind, compiled by Vite)
- **routes/web.php** - HTTP route definitions
- **config/** - Configuration files (app name, database, cache, etc.)
- **database/migrations, factories, seeders/** - Database layer
- **tests/Unit, tests/Feature/** - Test suites

### Key Technologies
- **Backend**: Laravel 12, PHP 8.2+
- **Frontend Build**: Vite 7 + Laravel Vite Plugin (auto-refresh on changes)
- **Styling**: Tailwind CSS 4 with @tailwindcss/vite
- **Database**: SQLite (in-memory for tests), Eloquent ORM
- **Authentication**: Built-in Laravel auth (User model with HasFactory, Notifiable)
- **Testing**: PHPUnit 11, Mockery, Faker

### Naming & Namespace Conventions
- Controllers: `App\Http\Controllers\{Name}Controller` → routes to `{route}`
- Models: `App\Models\{Name}` → automatically plural table names via Eloquent
- Requests (Form Validation): `App\Http\Requests\{Name}Request`
- Tests: `Tests\Unit\*` (isolated logic) and `Tests\Feature\*` (HTTP/integration)

### Database & Models
- Migrations use `Schema::create()` with timestamps (auto-added)
- Models use `$fillable` array to specify mass-assignable attributes
- Relationships use trait `HasFactory` and `Notifiable` (see User.php)
- Database connection via `DB_CONNECTION=sqlite` in `.env`

### Frontend Integration
- **Vite asset compilation**: Reference in Blade as `@vite(['resources/css/app.css', 'resources/js/app.js'])`
- **HMR (Hot Module Replacement)**: Auto-refreshes when `.blade.php` or JS/CSS files change
- **Tailwind**: Configured via `@tailwindcss/vite` (not traditional JIT)
- **Ignored on watch**: `**/storage/framework/views/**` to prevent rebuild loops

### Request Validation & Authorization
- Use custom `FormRequest` classes (e.g., `ProfileUpdateRequest`) with `rules()` method
- Access validated data via `$request->validated()` (returns array)
- Use `$request->validateWithBag()` for grouped validation errors
- Models provide auth context via `$request->user()`

### Blade Template Conventions
- Store in `resources/views/{feature}/{action}.blade.php` (e.g., `profile/edit.blade.php`)
- Pass data as associative array: `view('profile.edit', ['user' => $user])`
- Use Tailwind classes directly in templates (no additional CSS compilation step)

### Testing Patterns
- Feature tests inherit `Tests\TestCase` and test HTTP flows
- Unit tests are in `tests/Unit/` for isolated logic
- Use `RefreshDatabase` trait to reset DB between tests
- Faker generates fake data via `UserFactory`
- Tests run in-memory SQLite with config cache cleared

## Critical Developer Context

### Common Tasks
1. **Add a new page**: Create route in `routes/web.php` → Controller method → Blade view
2. **Add a model with CRUD**: Generate migration, Factory, Controller; define `$fillable`; test with Feature tests
3. **Update frontend styles**: Edit `resources/css/app.css` or inline Tailwind; Vite HMR auto-reloads
4. **Add form validation**: Create `{Name}Request` class with `rules()`; use in Controller method

### Gotchas & Conventions
- **Email verification**: Toggled via `email_verified_at` NULL check (see `ProfileController::update()`)
- **Password hashing**: Use `Hash::make()` in seeders/factories, never store plain text
- **Soft deletes**: Not enabled by default; add `SoftDeletes` trait + migration if needed
- **Configuration caching**: Run `config:clear` before tests or after config changes
- **Composer scripts**: All scripts use `@php` prefix to ensure cross-platform compatibility
- **View compilation**: Blade compiles to `storage/framework/views/` at runtime (ignored by Vite)

### Environment & Dependencies
- `.env` required (created from `.env.example` during setup)
- `node_modules/` and `vendor/` excluded from version control
- Packages auto-discovered via `package:discover` post-composer
- Vite watches `resources/` and `routes/` by default

## References
- [Laravel 12 Docs](https://laravel.com/docs/12)
- [Vite Plugin for Laravel](https://laravel.com/docs/12/vite)
- [Tailwind CSS with Vite](https://tailwindcss.com/blog/tailwindcss-v4-vite)
