package hclbuilder_test

import (
	"testing"

	"github.com/magodo/hclbuilder"
	"github.com/stretchr/testify/require"
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
	owner = "foo"
    environment = "dev"
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
    owner       = "bar"
    environment = "prod"
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
  subnet {
    name             = "subnet3"
    address_prefixes = ["10.10.3.0/24"]
  }
}
`
	t.Run("set attributes", func(t *testing.T) {
		b := hclbuilder.New([]byte(template))
		b.SetAt("[resource.azurerm_virtual_network.test].tags", `{
			owner = "bar"
			environment = "prod"
		}`)
		b.SetAt("[resource.azurerm_virtual_network.test].[encryption].enforcement", `"DenyUnencrypted"`)
		b.SetAt("[resource.azurerm_virtual_network.test].[subnet.1].address_prefixes", `["10.10.2.0/24"]`)
		b.SetAt("[resource.azurerm_virtual_network.test].location", `"westus2"`)
		b.SetAt("[resource.azurerm_virtual_network.test].[subnet.1]", `subnet {
			name = "subnet3"
			address_prefixes = ["10.10.3.0/24"]
		}
		`)
		require.Equal(t, expected, b.BuildString())
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
	require.Equal(t, expected, b.BuildString())
}
