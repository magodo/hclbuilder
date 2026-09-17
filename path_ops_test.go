package hclbuilder_test

import (
	"testing"

	"github.com/magodo/hclbuilder"
)

const template = `
resource "azurerm_resource_group" "test"{
	name     = "test"
	location = "centrual us"
}

resource "azurerm_virtual_network" "test" {
  name                = "test"
  address_space       = ["10.0.0.0/16", "10.10.0.0/16"]
  resource_group_name = azurerm_resource_group.test.name
  tags = {
    environment = "dev"
    owner       = "network"
  }

  encryption {
    enforcement = "AllowUnencrypted"
  }

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

  subnet {
    name             = "subnet2"
    address_prefixes = ["10.10.1.0/24"]
  }
}
`

func TestSetAt(t *testing.T) {
	expected := `
resource "azurerm_resource_group" "test" {
  name     = "test"
  location = "centrual us"
}

resource "azurerm_virtual_network" "test" {
  name                = "test"
  address_space       = ["10.0.0.0/16", "10.10.0.0/16"]
  resource_group_name = azurerm_resource_group.test.name
  tags = {
    environment = "prod"
    owner       = "network"
    cost_center = "1234"
  }

  encryption {
    enforcement = "DenyUnencrypted"
  }

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

  subnet {
    name             = "subnet2"
    address_prefixes = ["10.10.2.0/24"]
  }
  location = "westus2"
}
`
	t.Run("set attributes", func(t *testing.T) {
		b := hclbuilder.New([]byte(template))
		b.SetAt("[resource.azurerm_virtual_network.test].tags.environment", `"prod"`)
		b.SetAt("[resource.azurerm_virtual_network.test].tags.cost_center", `"1234"`)
		b.SetAt("[resource.azurerm_virtual_network.test].[encryption].enforcement", `"DenyUnencrypted"`)
		b.SetAt("[resource.azurerm_virtual_network.test].[subnet.1].address_prefixes", `["10.10.2.0/24"]`)
		b.SetAt("[resource.azurerm_virtual_network.test].location", `"westus2"`)

		if result := b.Build(); string(result) != expected {
			t.Errorf("wrong result:\n%s", string(result))
		}
	})
}

func TestRemoveAt(t *testing.T) {
	expected := `

resource "azurerm_virtual_network" "test" {
  name          = "test"
  address_space = ["10.0.0.0/16", "10.10.0.0/16"]
  tags = {
    environment = "dev"
  }

  encryption {
  }

  subnet {
    name             = "subnet1"
    address_prefixes = ["10.0.1.0/24"]
  }

}
`
	b := hclbuilder.New([]byte(template))
	b.RemoveAt("[resource.azurerm_resource_group.test]")
	b.RemoveAt("[resource.azurerm_virtual_network.test].resource_group_name")
	b.RemoveAt("[resource.azurerm_virtual_network.test].tags.owner")
	b.RemoveAt("[resource.azurerm_virtual_network.test].[encryption].enforcement")
	b.RemoveAt("[resource.azurerm_virtual_network.test].[subnet.1]")

	if result := b.BuildString(); result != expected {
		t.Errorf("wrong result:\n%s", result)
	}
}
