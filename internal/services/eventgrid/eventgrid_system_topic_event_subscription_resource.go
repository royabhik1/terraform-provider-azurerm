package eventgrid

import (
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/preview/eventgrid/mgmt/2020-10-15-preview/eventgrid"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/eventgrid/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func PossibleSystemTopicEventSubscriptionEndpointTypes() []string {
	return []string{
		string(AzureFunctionEndpoint),
		string(EventHubEndpointID),
		string(HybridConnectionEndpointID),
		string(ServiceBusQueueEndpointID),
		string(ServiceBusTopicEndpointID),
		string(StorageQueueEndpoint),
		string(WebHookEndpoint),
	}
}

func resourceEventGridSystemTopicEventSubscription() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceEventGridSystemTopicEventSubscriptionCreateUpdate,
		Read:   resourceEventGridSystemTopicEventSubscriptionRead,
		Update: resourceEventGridSystemTopicEventSubscriptionCreateUpdate,
		Delete: resourceEventGridSystemTopicEventSubscriptionDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.SystemTopicEventSubscriptionID(id)
			return err
		}),

		Schema: map[string]*pluginsdk.Schema{
			"name": eventSubscriptionSchemaEventSubscriptionName(),

			"system_topic": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"event_delivery_schema": eventSubscriptionSchemaEventDeliverySchema(),

			"expiration_time_utc": eventSubscriptionSchemaExpirationTimeUTC(),

			"azure_function_endpoint": eventSubscriptionSchemaAzureFunctionEndpoint(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(AzureFunctionEndpoint),
				),
			),

			"eventhub_endpoint_id": eventSubscriptionSchemaEventHubEndpointID(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(EventHubEndpointID),
				),
			),

			"hybrid_connection_endpoint_id": eventSubscriptionSchemaHybridConnectionEndpointID(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(HybridConnectionEndpointID),
				),
			),

			"service_bus_queue_endpoint_id": eventSubscriptionSchemaServiceBusQueueEndpointID(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(ServiceBusQueueEndpointID),
				),
			),

			"service_bus_topic_endpoint_id": eventSubscriptionSchemaServiceBusTopicEndpointID(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(ServiceBusTopicEndpointID),
				),
			),

			"storage_queue_endpoint": eventSubscriptionSchemaStorageQueueEndpoint(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(StorageQueueEndpoint),
				),
			),

			"webhook_endpoint": eventSubscriptionSchemaWebHookEndpoint(
				utils.RemoveFromStringArray(
					PossibleSystemTopicEventSubscriptionEndpointTypes(),
					string(WebHookEndpoint),
				),
			),

			"included_event_types": eventSubscriptionSchemaIncludedEventTypes(),

			"subject_filter": eventSubscriptionSchemaSubjectFilter(),

			"advanced_filter": eventSubscriptionSchemaAdvancedFilter(),

			"delivery_identity": eventSubscriptionSchemaIdentity(),

			"dead_letter_identity": eventSubscriptionSchemaIdentity(),

			"storage_blob_dead_letter_destination": eventSubscriptionSchemaStorageBlobDeadletterDestination(),

			"retry_policy": eventSubscriptionSchemaRetryPolicy(),

			"labels": eventSubscriptionSchemaLabels(),

			"advanced_filtering_on_arrays_enabled": eventSubscriptionSchemaEnableAdvancedFilteringOnArrays(),
		},
	}
}

func resourceEventGridSystemTopicEventSubscriptionCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).EventGrid.SystemTopicEventSubscriptionsClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)
	systemTopic := d.Get("system_topic").(string)

	if d.IsNewResource() {
		existing, err := client.Get(ctx, resourceGroup, systemTopic, name)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("checking for presence of existing EventGrid System Topic Event Subscription %q (System Topic %q): %s", name, systemTopic, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_eventgrid_system_topic_event_subscription", *existing.ID)
		}
	}

	destination := expandEventGridEventSubscriptionDestination(d)
	if destination == nil {
		return fmt.Errorf("One of the following endpoint types must be specificed to create an EventGrid System Topic Event Subscription: %q", PossibleSystemTopicEventSubscriptionEndpointTypes())
	}

	filter, err := expandEventGridEventSubscriptionFilter(d)
	if err != nil {
		return fmt.Errorf("expanding filters for EventGrid System Topic Event Subscription %q (System Topic %q): %+v", name, systemTopic, err)
	}

	expirationTime, err := expandEventGridExpirationTime(d)
	if err != nil {
		return fmt.Errorf("creating/updating EventGrid System Topic Event Subscription %q (System Topic %q): %s", name, systemTopic, err)
	}

	deadLetterDestination := expandEventGridEventSubscriptionStorageBlobDeadLetterDestination(d)

	eventSubscriptionProperties := eventgrid.EventSubscriptionProperties{
		Filter:              filter,
		RetryPolicy:         expandEventGridEventSubscriptionRetryPolicy(d),
		Labels:              utils.ExpandStringSlice(d.Get("labels").([]interface{})),
		EventDeliverySchema: eventgrid.EventDeliverySchema(d.Get("event_delivery_schema").(string)),
		ExpirationTimeUtc:   expirationTime,
	}

	if v, ok := d.GetOk("delivery_identity"); ok {
		deliveryIdentityRaw := v.([]interface{})
		deliveryIdentity, err := expandEventGridEventSubscriptionIdentity(deliveryIdentityRaw)
		if err != nil {
			return fmt.Errorf("expanding `delivery_identity`: %+v", err)
		}

		eventSubscriptionProperties.DeliveryWithResourceIdentity = &eventgrid.DeliveryWithResourceIdentity{
			Identity:    deliveryIdentity,
			Destination: destination,
		}
	} else {
		eventSubscriptionProperties.Destination = destination
	}

	if v, ok := d.GetOk("dead_letter_identity"); ok {
		if deadLetterDestination == nil {
			return fmt.Errorf("`dead_letter_identity`: `storage_blob_dead_letter_destination` must be specified")
		}
		deadLetterIdentityRaw := v.([]interface{})
		deadLetterIdentity, err := expandEventGridEventSubscriptionIdentity(deadLetterIdentityRaw)
		if err != nil {
			return fmt.Errorf("expanding `dead_letter_identity`: %+v", err)
		}

		eventSubscriptionProperties.DeadLetterWithResourceIdentity = &eventgrid.DeadLetterWithResourceIdentity{
			Identity:              deadLetterIdentity,
			DeadLetterDestination: deadLetterDestination,
		}
	} else {
		eventSubscriptionProperties.DeadLetterDestination = deadLetterDestination
	}

	eventSubscription := eventgrid.EventSubscription{
		EventSubscriptionProperties: &eventSubscriptionProperties,
	}

	log.Printf("[INFO] preparing arguments for AzureRM EventGrid System Topic Event Subscription creation with Properties: %+v.", eventSubscription)

	future, err := client.CreateOrUpdate(ctx, resourceGroup, systemTopic, name, eventSubscription)
	if err != nil {
		return fmt.Errorf("creating/updating EventGrid System Topic Event Subscription %q (System Topic %q): %s", name, systemTopic, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for EventGrid System Topic Event Subscription %q (System Topic %q) to become available: %s", name, systemTopic, err)
	}

	read, err := client.Get(ctx, resourceGroup, systemTopic, name)
	if err != nil {
		return fmt.Errorf("retrieving EventGrid System Topic Event Subscription %q (System Topic %q): %s", name, systemTopic, err)
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read EventGrid System Topic Event Subscription %s (System Topic %s) ID", name, systemTopic)
	}

	d.SetId(*read.ID)

	return resourceEventGridSystemTopicEventSubscriptionRead(d, meta)
}

func resourceEventGridSystemTopicEventSubscriptionRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).EventGrid.SystemTopicEventSubscriptionsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SystemTopicEventSubscriptionID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, id.ResourceGroup, id.SystemTopic, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[WARN] EventGrid System Topic Event Subscription '%q' was not found (System Topic %q)", id.Name, id.SystemTopic)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("making Read request on EventGrid System Topic Event Subscription '%q' (System Topic %q): %+v", id.Name, id.SystemTopic, err)
	}

	d.Set("name", resp.Name)
	d.Set("system_topic", id.SystemTopic)
	d.Set("resource_group_name", id.ResourceGroup)

	if props := resp.EventSubscriptionProperties; props != nil {
		if props.ExpirationTimeUtc != nil {
			d.Set("expiration_time_utc", props.ExpirationTimeUtc.Format(time.RFC3339))
		}

		d.Set("event_delivery_schema", string(props.EventDeliverySchema))

		destination := props.Destination
		deliveryIdentityFlattened := make([]interface{}, 0)
		if deliveryIdentity := props.DeliveryWithResourceIdentity; deliveryIdentity != nil {
			destination = deliveryIdentity.Destination
			deliveryIdentityFlattened = flattenEventGridEventSubscriptionIdentity(deliveryIdentity.Identity)
		}
		if err := d.Set("delivery_identity", deliveryIdentityFlattened); err != nil {
			return fmt.Errorf("setting `delivery_identity` for EventGrid System Topic Event Subscription %q (System Topic  %q): %s", id.Name, id.SystemTopic, err)
		}

		if azureFunctionEndpoint, ok := destination.AsAzureFunctionEventSubscriptionDestination(); ok {
			if err := d.Set("azure_function_endpoint", flattenEventGridEventSubscriptionAzureFunctionEndpoint(azureFunctionEndpoint)); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "azure_function_endpoint", id.Name, id.SystemTopic, err)
			}
		}
		if v, ok := destination.AsEventHubEventSubscriptionDestination(); ok {
			if err := d.Set("eventhub_endpoint_id", v.ResourceID); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "eventhub_endpoint_id", id.Name, id.SystemTopic, err)
			}
		}
		if v, ok := destination.AsHybridConnectionEventSubscriptionDestination(); ok {
			if err := d.Set("hybrid_connection_endpoint_id", v.ResourceID); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "hybrid_connection_endpoint_id", id.Name, id.SystemTopic, err)
			}
		}
		if serviceBusQueueEndpoint, ok := destination.AsServiceBusQueueEventSubscriptionDestination(); ok {
			if err := d.Set("service_bus_queue_endpoint_id", serviceBusQueueEndpoint.ResourceID); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "service_bus_queue_endpoint_id", id.Name, id.SystemTopic, err)
			}
		}
		if serviceBusTopicEndpoint, ok := destination.AsServiceBusTopicEventSubscriptionDestination(); ok {
			if err := d.Set("service_bus_topic_endpoint_id", serviceBusTopicEndpoint.ResourceID); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "service_bus_topic_endpoint_id", id.Name, id.SystemTopic, err)
			}
		}
		if v, ok := destination.AsStorageQueueEventSubscriptionDestination(); ok {
			if err := d.Set("storage_queue_endpoint", flattenEventGridEventSubscriptionStorageQueueEndpoint(v)); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "storage_queue_endpoint", id.Name, id.SystemTopic, err)
			}
		}
		if v, ok := destination.AsWebHookEventSubscriptionDestination(); ok {
			fullURL, err := client.GetFullURL(ctx, id.ResourceGroup, id.SystemTopic, id.Name)
			if err != nil {
				return fmt.Errorf("making Read request on EventGrid System Topic Event Subscription full URL '%s': %+v", id.Name, err)
			}
			if err := d.Set("webhook_endpoint", flattenEventGridEventSubscriptionWebhookEndpoint(v, &fullURL)); err != nil {
				return fmt.Errorf("setting `%q` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", "webhook_endpoint", id.Name, id.SystemTopic, err)
			}
		}

		deadLetterDestination := props.DeadLetterDestination
		deadLetterIdentityFlattened := make([]interface{}, 0)
		if deadLetterIdentity := props.DeadLetterWithResourceIdentity; deadLetterIdentity != nil {
			deadLetterDestination = deadLetterIdentity.DeadLetterDestination
			deadLetterIdentityFlattened = flattenEventGridEventSubscriptionIdentity(deadLetterIdentity.Identity)
		}
		if err := d.Set("dead_letter_identity", deadLetterIdentityFlattened); err != nil {
			return fmt.Errorf("setting `dead_letter_identity` for EventGrid System Topic Event Subscription %q (Scope %q): %s", id.Name, id.SystemTopic, err)
		}

		if deadLetterDestination != nil {
			if storageBlobDeadLetterDestination, ok := deadLetterDestination.AsStorageBlobDeadLetterDestination(); ok {
				if err := d.Set("storage_blob_dead_letter_destination", flattenEventGridEventSubscriptionStorageBlobDeadLetterDestination(storageBlobDeadLetterDestination)); err != nil {
					return fmt.Errorf("Error setting `storage_blob_dead_letter_destination` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", id.Name, id.SystemTopic, err)
				}
			}
		}

		if filter := props.Filter; filter != nil {
			d.Set("included_event_types", filter.IncludedEventTypes)
			d.Set("advanced_filtering_on_arrays_enabled", filter.EnableAdvancedFilteringOnArrays)
			if err := d.Set("subject_filter", flattenEventGridEventSubscriptionSubjectFilter(filter)); err != nil {
				return fmt.Errorf("setting `subject_filter` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", id.Name, id.SystemTopic, err)
			}
			if err := d.Set("advanced_filter", flattenEventGridEventSubscriptionAdvancedFilter(filter)); err != nil {
				return fmt.Errorf("setting `advanced_filter` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", id.Name, id.SystemTopic, err)
			}
		}

		if props.DeadLetterDestination != nil {
			if storageBlobDeadLetterDestination, ok := props.DeadLetterDestination.AsStorageBlobDeadLetterDestination(); ok {
				if err := d.Set("storage_blob_dead_letter_destination", flattenEventGridEventSubscriptionStorageBlobDeadLetterDestination(storageBlobDeadLetterDestination)); err != nil {
					return fmt.Errorf("setting `storage_blob_dead_letter_destination` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", id.Name, id.SystemTopic, err)
				}
			}
		}

		if retryPolicy := props.RetryPolicy; retryPolicy != nil {
			if err := d.Set("retry_policy", flattenEventGridEventSubscriptionRetryPolicy(retryPolicy)); err != nil {
				return fmt.Errorf("setting `retry_policy` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", id.Name, id.SystemTopic, err)
			}
		}

		if err := d.Set("labels", props.Labels); err != nil {
			return fmt.Errorf("setting `labels` for EventGrid System Topic Event Subscription %q (System Topic %q): %s", id.Name, id.SystemTopic, err)
		}
	}

	return nil
}

func resourceEventGridSystemTopicEventSubscriptionDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).EventGrid.SystemTopicEventSubscriptionsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SystemTopicEventSubscriptionID(d.Id())
	if err != nil {
		return err
	}

	future, err := client.Delete(ctx, id.ResourceGroup, id.SystemTopic, id.Name)
	if err != nil {
		return fmt.Errorf("deleting %s: %+v", *id, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for the deletion of %s: %+v", *id, err)
	}

	return nil
}
