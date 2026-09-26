import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
} from '@tanstack/react-router'
import { DevUserSwitcher } from './components/DevUserSwitcher'
import { LibraryPage } from './features/library/LibraryPage'
import { CourseListPage } from './features/builder/CourseListPage'
import { CourseBuilderPage } from './features/builder/CourseBuilderPage'
import { PlayerPage } from './features/player/PlayerPage'

const rootRoute = createRootRoute({
  component: () => (
    <div className="min-h-screen bg-white">
      <header className="border-b border-line">
        <nav className="mx-auto flex max-w-5xl items-center gap-6 px-6 py-3">
          <Link to="/" className="font-semibold tracking-tight">
            WikiPetia
          </Link>
          <Link to="/" className="text-sm text-muted [&.active]:text-ink">
            Library
          </Link>
          <Link to="/build" className="text-sm text-muted [&.active]:text-ink">
            Build
          </Link>
          <div className="ml-auto">
            <DevUserSwitcher />
          </div>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  ),
})

const libraryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: LibraryPage,
})

const courseListRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/build',
  component: CourseListPage,
})

const courseBuilderRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/build/$courseId',
  component: CourseBuilderPage,
})

const playerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/learn/$enrolmentId',
  component: PlayerPage,
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    libraryRoute,
    courseListRoute,
    courseBuilderRoute,
    playerRoute,
  ]),
  defaultPreload: 'intent',
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
