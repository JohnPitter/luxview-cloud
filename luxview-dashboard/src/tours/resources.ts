import type { TourStep } from '../components/common/PageTour';

export const resourcesTourSteps: TourStep[] = [
  {
    target: 'body',
    placement: 'center',
    title: 'tour.resources.welcome.title',
    content: 'tour.resources.welcome.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="services-list"]',
    title: 'tour.resources.services.title',
    content: 'tour.resources.services.content',
  },
  {
    target: '[data-tour="add-service"]',
    title: 'tour.resources.addService.title',
    content: 'tour.resources.addService.content',
  },
];
